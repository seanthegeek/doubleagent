//go:build parity

package pyjson

import (
	"bytes"
	"encoding/hex"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// oracleScript reads lines of a kind letter and a hex document. Kind s is
// text (UTF-8 with surrogates passed through) for DecodeString, kind b raw
// bytes for Decode. It prints, per line, SKIP for text Python cannot hold,
// ERR for a document it rejects, or the hex of the six encodings
// TestParityWithPython compares, separated by spaces.
const oracleScript = `import json, sys
def hexed(text):
    return text.encode("utf-8", "surrogatepass").hex()
for line in sys.stdin:
    kind, _, payload = line.rstrip("\n").partition(" ")
    data = bytes.fromhex(payload)
    try:
        text = data.decode("utf-8", "surrogatepass" if kind == "s" else "replace")
    except UnicodeDecodeError:
        print("SKIP")
        continue
    try:
        value = json.loads(text)
    except ValueError:
        print("ERR")
        continue
    print(" ".join(hexed(part) for part in (
        json.dumps(value, ensure_ascii=False, separators=(",", ":")),
        json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True),
        json.dumps(value),
        json.dumps(value, ensure_ascii=False, sort_keys=True),
        json.dumps(value, indent=2),
        str(value))))
`

// runOracle runs oracleScript under the interpreter PARITY_PYTHON names
// over input, skipping the test when the variable is unset.
func runOracle(t *testing.T, script string, input []byte) []string {
	t.Helper()
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	command := exec.Command(python, "-I", "-c", script)
	command.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	return strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
}

// goEncodings returns the six encodings oracleScript prints, as Go gives
// them, or ERR.
func goEncodings(value Value, err error) string {
	if err != nil {
		return "ERR"
	}
	parts := []string{Compact(value), Sorted(value), Spaced(value),
		Format{SortKeys: true, Spaced: true}.Encode(value), Indented(value), value.Str()}
	for i, part := range parts {
		parts[i] = hex.EncodeToString([]byte(part))
	}
	return strings.Join(parts, " ")
}

func TestParityWithPython(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	var documents []string
	for range 3000 {
		documents = append(documents, generateValue(random, 0))
	}
	for i := range 1000 {
		documents = append(documents, mutate(random, documents[i]))
	}
	var input bytes.Buffer
	for i, document := range documents {
		kind := "s"
		if i%3 == 0 {
			kind = "b"
		}
		input.WriteString(kind + " " + hex.EncodeToString([]byte(document)) + "\n")
	}
	want := runOracle(t, oracleScript, input.Bytes())
	if len(want) != len(documents) {
		t.Fatalf("oracle printed %d lines for %d documents", len(want), len(documents))
	}
	counts := map[string]int{}
	for i, document := range documents {
		counts[want[i][:min(len(want[i]), 4)]]++
		var got string
		if i%3 == 0 {
			got = goEncodings(Decode([]byte(document)))
		} else {
			got = goEncodings(DecodeString(document))
		}
		if want[i] == "SKIP" {
			continue
		}
		if got != want[i] {
			t.Errorf("document %q:\n got %s\nwant %s", document, got, want[i])
		}
	}
	t.Logf("compared %d documents: %d errors, %d skipped",
		len(documents), counts["ERR"], counts["SKIP"])
}

// generateValue returns a random JSON document whose nesting is at most
// four levels below depth 0.
func generateValue(random *rand.Rand, depth int) string {
	choice := random.IntN(10)
	if depth >= 4 && choice >= 8 {
		choice = random.IntN(8)
	}
	switch choice {
	case 0, 1, 2:
		return generateNumber(random)
	case 3, 4, 5:
		return generateString(random)
	case 6:
		return []string{"null", "true", "false", "NaN", "Infinity", "-Infinity"}[random.IntN(6)]
	case 7:
		return generateString(random)
	case 8:
		items := make([]string, random.IntN(5))
		for i := range items {
			items[i] = generateValue(random, depth+1)
		}
		return "[" + strings.Join(items, generateSeparator(random, ",")) + "]"
	}
	members := make([]string, random.IntN(12))
	for i := range members {
		members[i] = generateString(random) + generateSeparator(random, ":") +
			generateValue(random, depth+1)
	}
	return "{" + strings.Join(members, generateSeparator(random, ",")) + "}"
}

// generateSeparator returns separator with random JSON whitespace around it.
func generateSeparator(random *rand.Rand, separator string) string {
	spaces := []string{"", "", " ", "\n", "\t", "\r\n  "}
	return spaces[random.IntN(len(spaces))] + separator + spaces[random.IntN(len(spaces))]
}

// generateNumber returns a random number in one of the forms JSON allows.
func generateNumber(random *rand.Rand) string {
	sign := []string{"", "", "-"}[random.IntN(3)]
	digits := strconv.FormatUint(random.Uint64()>>random.IntN(64), 10)
	if random.IntN(8) == 0 {
		digits += strconv.FormatUint(random.Uint64(), 10) + strconv.FormatUint(random.Uint64(), 10)
	}
	switch random.IntN(4) {
	case 0:
		return sign + digits
	case 1:
		return sign + digits + "." + strconv.Itoa(random.IntN(100000))
	case 2:
		exponent := []string{"e", "E", "e+", "e-", "E-"}[random.IntN(5)]
		return sign + digits[:1] + exponent + strconv.Itoa(random.IntN(400))
	}
	return sign + "0." + strings.Repeat("0", random.IntN(12)) + digits + "e" +
		strconv.Itoa(random.IntN(40)-20)
}

// generateString returns a random JSON string mixing plain text, raw
// non-ASCII characters, short escapes and \u escapes, surrogates included.
func generateString(random *rand.Rand) string {
	pieces := []string{"a", "key", " ", "'", "\x7f", "\xc3\xa9", "\xe4\xb8\xad", "\xe2\x80\xa8",
		"\xf0\x9f\x98\x80", "\xf0\x9f\xaa\x8a", "\xc2\xa0", "\xc2\x85",
		"\\\"", "\\\\", "\\/", "\\b", "\\f", "\\n",
		"\\r", "\\t", "\\u0000", "\\u001F", "\\u00e9", "\\ud83d", "\\ude00", "\\uD83D\\uDE00",
		"\\udbff\\udfff", "\\ufeff", "\\u0378", "\\uffff", "\\u2028"}
	var out strings.Builder
	out.WriteByte('"')
	for range random.IntN(8) {
		out.WriteString(pieces[random.IntN(len(pieces))])
	}
	out.WriteByte('"')
	return out.String()
}

// mutate returns document with one random change: cut short, a byte
// replaced, a byte inserted or a trailing comma added.
func mutate(random *rand.Rand, document string) string {
	if document == "" {
		return ","
	}
	position := random.IntN(len(document))
	noise := []string{",", "]", "}", "\"", "\\", "0", "-", ".", "e", "\xff", "\x00", " ",
		"\xed\xa0\x80"}
	switch random.IntN(4) {
	case 0:
		return document[:position]
	case 1:
		return document[:position] + noise[random.IntN(len(noise))] + document[position+1:]
	case 2:
		return document[:position] + noise[random.IntN(len(noise))] + document[position:]
	}
	return document + ","
}

// printableScript prints the ranges of code points for which
// str.isprintable() holds, as "first-last" pairs separated by spaces.
const printableScript = `import sys
ranges = []
for code in range(sys.maxunicode + 1):
    if chr(code).isprintable():
        if ranges and ranges[-1][1] == code - 1:
            ranges[-1][1] = code
        else:
            ranges.append([code, code])
print(" ".join("%d-%d" % (first, last) for first, last in ranges))
`

func TestPrintableParityWithPython(t *testing.T) {
	want := runOracle(t, printableScript, nil)[0]
	var ranges []string
	first := rune(-1)
	for char := rune(0); char <= 0x110000; char++ {
		printable := char <= 0x10FFFF && isPrintable(char)
		switch {
		case printable && first < 0:
			first = char
		case !printable && first >= 0:
			ranges = append(ranges, strconv.Itoa(int(first))+"-"+strconv.Itoa(int(char-1)))
			first = -1
		}
	}
	if got := strings.Join(ranges, " "); got != want {
		t.Errorf("printable ranges differ from Python's:\n got %.300s\nwant %.300s", got, want)
	}
}
