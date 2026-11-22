package parserfacts

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// factsVariable names the environment variable holding the path the
// report is written to.
const factsVariable = "PARSERFACTS_OUT"

func TestWriteTheRegisteredParsersFacts(t *testing.T) {
	out := os.Getenv(factsVariable)
	if out == "" {
		t.Skip(factsVariable + " names no file to write the report to")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err == nil {
		err = writeFacts(root, out)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// writeFacts analyzes the registered parsers of the module at root and
// writes the report to the file out.
func writeFacts(root, out string) error {
	module, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	config := settings{root: root, module: module, tags: []string{"noasm", "nounsafe"},
		registry: module + "/internal/parsers",
		isAgent: func(path string) bool {
			rest, ok := strings.CutPrefix(path, module+"/internal/parser/")
			return ok && rest != "" && rest != "parsertest"
		}}
	found, err := analyze(config, registered())
	if err != nil {
		return err
	}
	file, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("write the report: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(found); err != nil {
		file.Close()
		return fmt.Errorf("write the report: %w", err)
	}
	return file.Close()
}

// registered returns the parsers of the registry as targets, with the
// answers each gives at run time about its secondary sources and marks.
func registered() []target {
	var found []target
	for _, each := range parsers.All() {
		kind := reflect.TypeOf(each)
		if kind.Kind() == reflect.Pointer {
			kind = kind.Elem()
		}
		marks := parser.MarksOf(each)
		found = append(found, target{Agent: each.Agent(), Package: kind.PkgPath(), TypeName: kind.Name(),
			Secondary: parser.HasSecondary(each), Sync: marks.Sync, Relay: marks.Relay, Origin: marks.Origin})
	}
	return found
}

// modulePath returns the module path a go.mod file declares.
func modulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read module path: %w", err)
	}
	defer file.Close()
	lines := bufio.NewScanner(file)
	for lines.Scan() {
		if name, ok := strings.CutPrefix(strings.TrimSpace(lines.Text()), "module "); ok {
			return strings.Trim(strings.TrimSpace(name), `"`), nil
		}
	}
	return "", errors.New("no module line in " + path)
}
