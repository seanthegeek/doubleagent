//go:build parity

package replay

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// corpusFormat is the format number of corpus.json this package reads.
const corpusFormat = 1

// Text is a string of the corpus: JSON text, or {"hex": ...} for bytes that
// are not valid UTF-8 (a lone surrogate the recorder wrote in its UTF-8 form,
// or a file name's raw bytes).
type Text string

// UnmarshalJSON reads either form of a Text.
func (text *Text) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		var tagged struct {
			Hex string `json:"hex"`
		}
		if err := json.Unmarshal(data, &tagged); err != nil {
			return fmt.Errorf("tagged text: %w", err)
		}
		raw, err := hex.DecodeString(tagged.Hex)
		if err != nil {
			return fmt.Errorf("tagged text: %w", err)
		}
		*text = Text(raw)
		return nil
	}
	var plain string
	if err := json.Unmarshal(data, &plain); err != nil {
		return fmt.Errorf("text: %w", err)
	}
	*text = Text(plain)
	return nil
}

// Header is corpus.json: the format, the placeholder of RootLength bytes that
// stands for the recorder's root inside a blob, and the tokens that stand for
// the root and the repository in the text of cases, trees and times.
type Header struct {
	Format          int    `json:"format"`
	RootLength      int    `json:"root_length"`
	RootPlaceholder string `json:"root_placeholder"`
	RootToken       string `json:"root_token"`
	RepoToken       string `json:"repo_token"`
}

// Corpus is an opened corpus directory.
type Corpus struct {
	Directory string
	Header    Header
}

// Open opens the corpus PARITY_CORPUS names, and skips the test when the
// variable is unset.
func Open(t testing.TB) *Corpus {
	t.Helper()
	directory := os.Getenv("PARITY_CORPUS")
	if directory == "" {
		t.Skip("PARITY_CORPUS is not set: record a corpus with the analyzer's recorder plugin " +
			"(pytest -p recorder.plugin) and name its directory")
	}
	corpus, err := OpenDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	return corpus
}

// OpenDirectory opens the corpus in directory and checks its format.
func OpenDirectory(directory string) (*Corpus, error) {
	data, err := osfile.ReadFile(filepath.Join(directory, "corpus.json"))
	if err != nil {
		return nil, fmt.Errorf("open corpus: %w", err)
	}
	corpus := &Corpus{Directory: directory}
	if err := json.Unmarshal(data, &corpus.Header); err != nil {
		return nil, fmt.Errorf("corpus.json: %w", err)
	}
	header := corpus.Header
	switch {
	case header.Format != corpusFormat:
		return nil, fmt.Errorf("corpus.json: format %d, this replay reads %d", header.Format,
			corpusFormat)
	case header.RootLength < 1 || len(header.RootPlaceholder) != header.RootLength:
		return nil, fmt.Errorf("corpus.json: a placeholder of %d bytes for a root of %d",
			len(header.RootPlaceholder), header.RootLength)
	case header.RootToken == "" || header.RepoToken == "":
		return nil, errors.New("corpus.json: no root or repository token")
	}
	return corpus, nil
}

// readCases decodes every line of a case file into a T, in file order.
func readCases[T any](path string) ([]T, error) {
	file, err := osfile.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read cases: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var cases []T
	for number := 1; ; number++ {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var value T
			if decodeErr := json.Unmarshal(line, &value); decodeErr != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, number, decodeErr)
			}
			cases = append(cases, value)
		}
		if errors.Is(err, io.EOF) {
			return cases, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, number, err)
		}
	}
}

// caseFile returns the path of a case file, failing the test when it is
// missing, since a misspelt agent or command would otherwise replay nothing.
func (c *Corpus) caseFile(t testing.TB, name string) string {
	t.Helper()
	path := filepath.Join(c.Directory, filepath.FromSlash(name)+".jsonl")
	if _, err := osfile.Lstat(path); err != nil {
		t.Fatalf("no cases %s in the corpus: %v", name, err)
	}
	return path
}

// ParserCases returns the parser cases of one agent, in call order.
func (c *Corpus) ParserCases(t testing.TB, agent string) []ParserCase {
	t.Helper()
	cases, err := readCases[ParserCase](c.caseFile(t, "parsers/"+agent))
	if err != nil {
		t.Fatal(err)
	}
	for i := range cases {
		if cases[i].ResultBlob == "" {
			continue
		}
		if err := c.readResult(cases[i].ResultBlob, &cases[i].Result); err != nil {
			t.Fatalf("%s: %v", cases[i].Node, err)
		}
	}
	return cases
}

// CommandCases returns the command cases of one command (detect, timeline,
// inventory, catalog, or other for a run whose first argument is none), in
// call order.
func (c *Corpus) CommandCases(t testing.TB, command string) []CommandCase {
	t.Helper()
	cases, err := readCases[CommandCase](c.caseFile(t, "commands/"+command))
	if err != nil {
		t.Fatal(err)
	}
	return cases
}

// InputCases returns the input cases, in call order, each artifact's home
// resolved.
func (c *Corpus) InputCases(t testing.TB) []InputCase {
	t.Helper()
	cases, err := readCases[InputCase](c.caseFile(t, "inputs"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range cases {
		if cases[i].ResultBlob != "" {
			cases[i].Result = &Collection{}
			if err := c.readResult(cases[i].ResultBlob, cases[i].Result); err != nil {
				t.Fatalf("%s: %v", cases[i].Node, err)
			}
		}
		if err := cases[i].resolveHomes(); err != nil {
			t.Fatalf("%s: %v", cases[i].Node, err)
		}
	}
	return cases
}

// readResult decodes the JSON of a result blob into result.
func (c *Corpus) readResult(digest string, result any) error {
	data, err := osfile.ReadFile(filepath.Join(c.Directory, "blobs", digest))
	if err != nil {
		return fmt.Errorf("read result: %w", err)
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("result %s: %w", digest, err)
	}
	return nil
}

// ParserCases opens the corpus and returns the parser cases of one agent.
func ParserCases(t testing.TB, agent string) []ParserCase {
	t.Helper()
	return Open(t).ParserCases(t, agent)
}

// CommandCases opens the corpus and returns the cases of one command.
func CommandCases(t testing.TB, command string) []CommandCase {
	t.Helper()
	return Open(t).CommandCases(t, command)
}

// InputCases opens the corpus and returns its input cases.
func InputCases(t testing.TB) []InputCase {
	t.Helper()
	return Open(t).InputCases(t)
}

// Expand returns the text of a case, a tree or times with the root token
// replaced by root (a directory Materialise returned) and the repository
// token by the repository root.
func (c *Corpus) Expand(text Text, root string) string {
	expanded := strings.ReplaceAll(string(text), c.Header.RootToken, root)
	return strings.ReplaceAll(expanded, c.Header.RepoToken, repositoryRoot())
}

// repositoryRoot returns the repository's absolute path, three levels above
// this package, or the relative form when the working directory is unknown.
func repositoryRoot() string {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		return filepath.Join("..", "..", "..")
	}
	return root
}
