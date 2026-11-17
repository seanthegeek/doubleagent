//go:build parity

package gitmeta

import (
	"bytes"
	"encoding/hex"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// oracleScript reads one hex-encoded text per line and prints, as one JSON
// line each, the analyzer's [parse_remotes(text, errors), errors] and then
// parse_head(text). The analyzer's directory is the first argument.
const oracleScript = `import json, sys
sys.path.insert(0, sys.argv[1])
from doubleagent.gitmeta import parse_head, parse_remotes
for line in sys.stdin:
    text = bytes.fromhex(line.strip()).decode("utf-8")
    errors = []
    remotes = parse_remotes(text, errors)
    print(json.dumps([remotes, errors, parse_head(text)]))
`

// textArray returns a JSON array of texts.
func textArray(texts []string) jsonvalue.Value {
	items := []jsonvalue.Value{}
	for _, text := range texts {
		items = append(items, jsonvalue.NewString(text))
	}
	return jsonvalue.NewArray(items...)
}

// encodeResult writes the Go results as the oracle prints them.
func encodeResult(text string) string {
	remotes, problems := ParseConfig(text)
	items := []jsonvalue.Value{}
	for _, found := range remotes {
		items = append(items, jsonvalue.NewObject(
			jsonvalue.Member{Key: "name", Value: jsonvalue.NewString(found.Name)},
			jsonvalue.Member{Key: "urls", Value: textArray(found.URLs)},
			jsonvalue.Member{Key: "pushurls", Value: textArray(found.PushURLs)}))
	}
	return jsonvalue.NewArray(jsonvalue.NewArray(items...), textArray(problems),
		jsonvalue.NewString(ParseHead(text))).JSON()

}

// randomConfig returns a config text built from fragments of headers,
// keys, separators and values that exercise the reader's rules.
func randomConfig(random *rand.Rand) string {
	pick := func(options ...string) string { return options[random.IntN(len(options))] }
	var text strings.Builder
	if random.IntN(10) == 0 {
		text.WriteString("\ufeff")
	}
	for range 1 + random.IntN(10) {
		text.WriteString(pick("", "  ", "\t", "\v"))
		switch random.IntN(5) {
		case 0:
			text.WriteString(pick(`[remote "origin"]`, `[remote.Fork]`, `[Remote "Up"]`,
				`[branch "main"]`, `[remote "a\"b\\c"]`, `[ remote "o" ]`, `[remote "o"`,
				`[core]`, `[remote ""]`, `[remote.]`, `[remote "x" ] url = h://inline`,
				`[remote	"tab"]`, `[remote"x"]`, `ref: refs/heads/topic`, `0123abc`))
		case 1:
			text.WriteString(pick("# comment \\", "; note", "", "= no key", "x"))
		default:
			text.WriteString(pick("url", "URL", "pushurl", "pushUrl", "fetch", "url2", "-url"))
			text.WriteString(pick(" = ", "=", " \t= ", " ", ""))
			for range random.IntN(5) {
				text.WriteString(pick("https://h/x", `"q ;#"`, `\t`, `\q`, `\\`, `\"`, " #c",
					";c", " ", `"`, "\\", "\u00e9", "\u2028", `\`+"\u00e9", "\r"))
			}
		}
		text.WriteString(pick("\n", "\n", "\r\n", ""))
	}
	return text.String()
}

func TestRandomConfigsMatchPython(t *testing.T) {
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	analyzer, err := filepath.Abs(filepath.Join("..", "..", "analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewPCG(43, 47))
	texts := make([]string, 5000)
	var input bytes.Buffer
	for i := range texts {
		texts[i] = randomConfig(random)
		input.WriteString(hex.EncodeToString([]byte(texts[i])) + "\n")
	}
	command := exec.Command(python, "-I", "-c", oracleScript, analyzer)
	command.Stdin = &input
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	answers := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(answers) != len(texts) {
		t.Fatalf("oracle gave %d answers for %d texts", len(answers), len(texts))
	}
	for i, answer := range answers {
		if got := encodeResult(texts[i]); got != answer {
			t.Errorf("config %q:\nGo     %s\nPython %s", texts[i], got, answer)
		}
	}
}
