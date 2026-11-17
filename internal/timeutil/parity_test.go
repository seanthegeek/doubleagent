//go:build parity

package timeutil

import (
	"bytes"
	"encoding/hex"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// oracleScript reads lines of a kind letter and a hex payload and prints
// the analyzer's answer for each: kind t is text for to_utc, f the eight
// big-endian bytes of a float for to_utc, i the decimal text of an integer
// for to_utc, and m text for epoch_ms (None printed as N). The analyzer's
// directory is the first argument, since -I ignores PYTHONPATH.
const oracleScript = `import struct, sys
sys.path.insert(0, sys.argv[1])
from doubleagent.timeutil import to_utc, epoch_ms
for line in sys.stdin:
    kind, _, payload = line.rstrip("\n").partition(" ")
    data = bytes.fromhex(payload)
    if kind == "t":
        print(to_utc(data.decode("utf-8")))
    elif kind == "f":
        print(to_utc(struct.unpack(">d", data)[0]))
    elif kind == "i":
        print(to_utc(int(data.decode("ascii"))))
    else:
        value = epoch_ms(data.decode("utf-8"))
        print("N" if value is None else value)
`

// query is one input for the oracle.
type query struct {
	kind    byte
	payload []byte
}

// askPython runs oracleScript under the interpreter PARITY_PYTHON names,
// skipping the test when the variable is unset, and returns one answer per
// query.
func askPython(t *testing.T, queries []query) []string {
	t.Helper()
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	analyzer, err := filepath.Abs(filepath.Join("..", "..", "analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	var input bytes.Buffer
	for _, item := range queries {
		input.WriteByte(item.kind)
		input.WriteByte(' ')
		input.WriteString(hex.EncodeToString(item.payload))
		input.WriteByte('\n')
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
	if len(answers) != len(queries) {
		t.Fatalf("oracle gave %d answers for %d queries", len(answers), len(queries))
	}
	return answers
}

// floatQuery returns the query for a float epoch.
func floatQuery(epoch float64) query {
	payload := make([]byte, 8)
	bits := math.Float64bits(epoch)
	for i := range payload {
		payload[i] = byte(bits >> (56 - 8*i))
	}
	return query{'f', payload}
}

func TestTablesMatchPython(t *testing.T) {
	var queries []query
	var stored []string
	for _, test := range textCases() {
		queries = append(queries, query{'t', []byte(test.input)})
		stored = append(stored, test.python)
	}
	for _, test := range epochCases() {
		queries = append(queries, floatQuery(math.Float64frombits(test.bits)))
		stored = append(stored, test.python)
	}
	for _, test := range intCases() {
		queries = append(queries, query{'i', []byte(strconv.FormatInt(test.input, 10))})
		stored = append(stored, test.python)
	}
	for _, test := range millisCases() {
		queries = append(queries, query{'m', []byte(test.input)})
		stored = append(stored, map[bool]string{true: strconv.FormatInt(test.python, 10),
			false: "N"}[test.ok])
	}
	for i, answer := range askPython(t, queries) {
		if answer != stored[i] {
			t.Errorf("query %c %q: Python now gives %q, the table holds %q", queries[i].kind,
				queries[i].payload, answer, stored[i])
		}
	}
}

func TestFloatEpochsMatchPython(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 5))
	var epochs []float64
	for range 2000 {
		epochs = append(epochs, random.Float64()*1.2e11-7e10)
	}
	for range 2000 {
		epochs = append(epochs, random.Float64()*2.6e14)
	}
	for range 1000 {
		// Halfway microsecond and millisecond fractions, where rounding shows.
		whole := float64(random.IntN(2_000_000_000))
		epochs = append(epochs, whole+float64(random.IntN(1_000_000))/1e6+5e-7,
			whole*1000+float64(random.IntN(1000))+0.5)
	}
	for range 1000 {
		epochs = append(epochs, math.Float64frombits(random.Uint64()))
	}
	queries := make([]query, len(epochs))
	for i, epoch := range epochs {
		queries[i] = floatQuery(epoch)
	}
	for i, answer := range askPython(t, queries) {
		if got := FromEpoch(epochs[i]); got != answer {
			t.Errorf("FromEpoch(%v) = %q, Python gives %q", epochs[i], got, answer)
		}
	}
}

func TestAcceptedFormsMatchPython(t *testing.T) {
	random := rand.New(rand.NewPCG(7, 11))
	texts := make([]string, 4000)
	for i := range texts {
		texts[i] = acceptedForm(random)
	}
	compareTexts(t, texts, true)
}

func TestOtherTextNeverDisagreesWithPython(t *testing.T) {
	random := rand.New(rand.NewPCG(13, 17))
	texts := make([]string, 6000)
	for i := range texts {
		texts[i] = mutate(random, acceptedForm(random))
	}
	compareTexts(t, texts, false)
}

// compareTexts runs FromText and the oracle over texts. Every non-empty Go
// result must equal Python's; with exact set, an empty one must too.
func compareTexts(t *testing.T, texts []string, exact bool) {
	t.Helper()
	queries := make([]query, len(texts))
	for i, text := range texts {
		queries[i] = query{'t', []byte(text)}
	}
	refused := 0
	for i, answer := range askPython(t, queries) {
		got := FromText(texts[i])
		switch {
		case got == answer:
		case got == "" && !exact:
			refused++
			if testing.Verbose() {
				t.Logf("refused %q, which Python reads as %q", texts[i], answer)
			}
		default:
			t.Errorf("FromText(%q) = %q, Python gives %q", texts[i], got, answer)
		}
	}
	t.Logf("%d of %d texts Python reads are refused here", refused, len(texts))
}

// acceptedForm returns a random timestamp in one of the forms the package
// accepts, its fields sometimes out of range.
func acceptedForm(random *rand.Rand) string {
	pick := func(options ...string) string { return options[random.IntN(len(options))] }
	number := func(limit, width int) string {
		text := strconv.Itoa(random.IntN(limit))
		return strings.Repeat("0", max(0, width-len(text))) + text
	}
	text := number(10000, 4) + "-" + number(14, 2) + "-" + number(33, 2)
	if random.IntN(6) == 0 {
		return pick("", " ", "\t", "\n") + text + pick("", "Z", "z", " ")
	}
	text += pick("T", "t", " ", "_", "/", ":", "x", "-") + number(25, 2) + ":" + number(61, 2)
	if random.IntN(4) > 0 {
		text += ":" + number(61, 2)
		if random.IntN(2) == 0 {
			text += "." + number(1_000_000_000_000, 12)[:1+random.IntN(12)]
		}
	}
	switch random.IntN(5) {
	case 0:
		text += pick("Z", "z")
	case 1:
		text += pick("+", "-") + number(25, 2) + pick(":", "") + number(60, 2)
	case 2:
		text += pick("+", "-") + number(25, 2)
	}
	return text
}

// mutate changes a few bytes of text: a replacement by a byte from a set
// that timestamps hold, an insertion or a deletion.
func mutate(random *rand.Rand, text string) string {
	const alphabet = "0123456789-:.,+ TZzW\x00\t"
	data := []byte(text)
	for range 1 + random.IntN(3) {
		position := random.IntN(len(data) + 1)
		char := alphabet[random.IntN(len(alphabet))]
		switch random.IntN(3) {
		case 0:
			if position < len(data) {
				data[position] = char
			}
		case 1:
			data = append(data[:position], append([]byte{char}, data[position:]...)...)
		default:
			if position < len(data) {
				data = append(data[:position], data[position+1:]...)
			}
		}
	}
	return string(data)
}
