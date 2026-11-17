//go:build parity

package pytext

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// oracleScript reads lines of a kind letter and hex data and prints one
// answer per line: kind f is the eight little-endian bytes of a float, its
// answer repr(float); kind u is bytes, its answer the hex of their decoding
// with replacement; kind s (no data) answers the code points for which
// str.isspace() holds, in decimal, separated by spaces.
const oracleScript = `import struct, sys
for line in sys.stdin:
    kind, _, payload = line.rstrip("\n").partition(" ")
    data = bytes.fromhex(payload)
    if kind == "f":
        print(repr(struct.unpack("<d", data)[0]))
    elif kind == "u":
        print(data.decode("utf-8", "replace").encode("utf-8").hex())
    else:
        print(" ".join(str(c) for c in range(sys.maxunicode + 1) if chr(c).isspace()))
`

// runOracle runs oracleScript under the interpreter PARITY_PYTHON names
// over input, skipping the test when the variable is unset.
func runOracle(t *testing.T, input []byte) []string {
	t.Helper()
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	command := exec.Command(python, "-I", "-c", oracleScript)
	command.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	return strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
}

// randomFloats returns floats from random bit patterns and from random
// decimal text, the second kind landing near round numbers where the
// choice between positional and exponent form is made.
func randomFloats(random *rand.Rand) []float64 {
	var floats []float64
	for range 3000 {
		floats = append(floats, math.Float64frombits(random.Uint64()))
	}
	for range 3000 {
		text := strconv.Itoa(random.IntN(100000)) + "e" + strconv.Itoa(random.IntN(50)-25)
		value, err := strconv.ParseFloat(text, 64)
		if err == nil {
			floats = append(floats, value)
		}
	}
	return floats
}

func TestFloatParityWithPython(t *testing.T) {
	floats := randomFloats(rand.New(rand.NewPCG(3, 4)))
	var input bytes.Buffer
	for _, value := range floats {
		input.WriteString("f " + hex.EncodeToString(binary.LittleEndian.AppendUint64(nil,
			math.Float64bits(value))) + "\n")
	}
	want := runOracle(t, input.Bytes())
	for i, value := range floats {
		if got := Float(value); got != want[i] {
			t.Errorf("Float(%b) = %q, want %q", value, got, want[i])
		}
	}
	t.Logf("compared %d floats", len(floats))
}

func TestDecodeUTF8ParityWithPython(t *testing.T) {
	random := rand.New(rand.NewPCG(5, 6))
	pieces := [][]byte{{0x80}, {0xBF}, {0xC0}, {0xC2}, {0xDF}, {0xE0}, {0xE0, 0xA0}, {0xED},
		{0xED, 0xA0}, {0xED, 0x9F}, {0xEF, 0xBF}, {0xF0}, {0xF0, 0x90}, {0xF0, 0x90, 0x80},
		{0xF4, 0x8F}, {0xF4, 0x90}, {0xF5}, {0xFF}, []byte("a"), []byte("\xc3\xa9"), []byte("\U0001F600")}
	var samples [][]byte
	for range 3000 {
		var sample []byte
		for range random.IntN(8) {
			sample = append(sample, pieces[random.IntN(len(pieces))]...)
		}
		samples = append(samples, sample)
	}
	var input bytes.Buffer
	for _, sample := range samples {
		input.WriteString("u " + hex.EncodeToString(sample) + "\n")
	}
	want := runOracle(t, input.Bytes())
	for i, sample := range samples {
		if got := hex.EncodeToString([]byte(DecodeUTF8(sample))); got != want[i] {
			t.Errorf("DecodeUTF8(%x) = %s, want %s", sample, got, want[i])
		}
	}
}

func TestStripSetParityWithPython(t *testing.T) {
	want := runOracle(t, []byte("s\n"))[0]
	var got []string
	for char := rune(0); char <= 0x10FFFF; char++ {
		if isPythonSpace(char) {
			got = append(got, strconv.Itoa(int(char)))
		}
	}
	if strings.Join(got, " ") != want {
		t.Errorf("space set = %v, want %s", got, want)
	}
}
