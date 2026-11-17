package zstdframe

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
)

// frameCase is one line of testdata/frames.txt: a frame the Python zstandard
// package wrote, and the hash and size of what it decodes to.
type frameCase struct {
	name   string
	digest string
	size   int
	frame  []byte
}

func loadFrames(t testing.TB) []frameCase {
	t.Helper()
	file, err := os.Open("testdata/frames.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var cases []frameCase
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		size, err := strconv.Atoi(parts[2])
		if err != nil {
			t.Fatal(err)
		}
		frame, err := os.ReadFile(filepath.Join("testdata", "frames", parts[0]+".zst"))
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, frameCase{parts[0], parts[1], size, frame})
	}
	return cases
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestDecodeAllMatchesWhatPythonCompressed(t *testing.T) {
	for _, test := range loadFrames(t) {
		if test.name == "too-wide-window" {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			got, err := DecodeAll(test.frame, 1<<30)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != test.size || digestOf(got) != test.digest {
				t.Fatalf("decoded %d bytes with digest %s, want %d bytes with %s",
					len(got), digestOf(got), test.size, test.digest)
			}
		})
	}
}

func TestDecodeStreamEmitsTheSameBytesFromASlowReader(t *testing.T) {
	for _, test := range loadFrames(t) {
		if test.size > 1<<20 || test.name == "too-wide-window" {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			var got bytes.Buffer
			source := iotest.OneByteReader(bytes.NewReader(test.frame))
			err := DecodeStream(source, func(chunk []byte) error {
				got.Write(chunk)
				return nil
			})
			if err != nil || digestOf(got.Bytes()) != test.digest {
				t.Fatalf("err %v, digest %s want %s", err, digestOf(got.Bytes()), test.digest)
			}
		})
	}
}

func TestFramesCoverTheShapesTheAgentsWrite(t *testing.T) {
	want := map[string]bool{"size1000-nosize": false, "multi-frame": false, "far-reference": false, "too-wide-window": false}
	for _, test := range loadFrames(t) {
		if _, ok := want[test.name]; ok {
			want[test.name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("frames.txt lacks %s", name)
		}
	}
	nosize := loadNamed(t, "size1000-nosize")
	if nosize[4]&0xC0 != 0 || nosize[4]&0x20 != 0 {
		t.Fatalf("the no-content-size frame has a size field: descriptor %#x", nosize[4])
	}
}

func loadNamed(t testing.TB, name string) []byte {
	t.Helper()
	for _, test := range loadFrames(t) {
		if test.name == name {
			return test.frame
		}
	}
	t.Fatalf("no frame %s", name)
	return nil
}

func TestEmptyInputIsAnEmptyStream(t *testing.T) {
	got, err := DecodeAll(nil, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %q %v", got, err)
	}
	if err := DecodeStream(bytes.NewReader(nil), func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestEveryProperPrefixOfAFrameIsTruncatedNotCorrupt(t *testing.T) {
	for _, name := range []string{"size1000-level3", "size1000-checksum", "size1000-nosize", "size1-level19", "multi-frame"} {
		frame := loadNamed(t, name)
		boundary := frameBoundaries(t, frame)
		step := 1 + len(frame)/3000
		for cut := 1; cut < len(frame); cut += step {
			_, err := DecodeAll(frame[:cut], 1<<30)
			if boundary[cut] {
				if err != nil {
					t.Fatalf("%s cut at the frame boundary %d: %v", name, cut, err)
				}
				continue
			}
			if !errors.Is(err, ErrTruncated) || errors.Is(err, ErrCorrupt) {
				t.Fatalf("%s cut at %d of %d: got %v", name, cut, len(frame), err)
			}
		}
	}
}

// frameBoundaries returns the offsets at which a prefix of data ends exactly
// after a whole frame, found by decoding the frames one at a time.
func frameBoundaries(t testing.TB, data []byte) map[int]bool {
	t.Helper()
	boundary := map[int]bool{}
	step := 1 + len(data)/3000
	for cut := 1; cut < len(data); cut += step {
		if _, err := DecodeAll(data[:cut], 1<<30); err == nil {
			boundary[cut] = true
		}
	}
	return boundary
}

func TestCorruptDataIsNotReportedAsTruncated(t *testing.T) {
	good := loadNamed(t, "size1000-checksum")
	for name, data := range map[string][]byte{
		"a wrong magic number":  append([]byte{0, 0, 0, 0}, good[4:]...),
		"garbage after a frame": append(append([]byte{}, good...), 1, 2, 3, 4, 5, 6),
		"a bad checksum":        flip(good, len(good)-1),
		"a reserved bit":        flip2(good, 4, 0x08),
		"a bad block type":      flip2(good, 6, 0x06),
		"a dictionary id":       flip2(good, 4, 0x03),
		"random bytes":          bytes.Repeat([]byte{0xA5}, 64),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeAll(data, 1<<30)
			if !errors.Is(err, ErrCorrupt) || errors.Is(err, ErrTruncated) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func flip(data []byte, place int) []byte {
	return flip2(data, place, 0x01)
}

func flip2(data []byte, place int, mask byte) []byte {
	out := append([]byte{}, data...)
	out[place] ^= mask
	return out
}

func TestOutputBeyondTheLimitIsRefused(t *testing.T) {
	frame := loadNamed(t, "size70000-level3")
	if _, err := DecodeAll(frame, 69999); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
	if got, err := DecodeAll(frame, 70000); err != nil || len(got) != 70000 {
		t.Fatalf("got %d bytes, %v", len(got), err)
	}
}

func TestOutputBeforeDamageIsStillEmitted(t *testing.T) {
	frame := loadNamed(t, "size300000-level3")
	var got int
	err := DecodeStream(bytes.NewReader(frame[:len(frame)/2]), func(chunk []byte) error {
		got += len(chunk)
		return nil
	})
	if !errors.Is(err, ErrTruncated) || got == 0 || got >= 300000 {
		t.Fatalf("emitted %d bytes, err %v", got, err)
	}
}

func TestAFailingReaderIsNeitherCorruptNorTruncated(t *testing.T) {
	broken := errors.New("disk fell over")
	source := io.MultiReader(bytes.NewReader(loadNamed(t, "size1000-level3")[:20]), iotest.ErrReader(broken))
	err := DecodeStream(source, func([]byte) error { return nil })
	if !errors.Is(err, broken) || errors.Is(err, ErrCorrupt) || errors.Is(err, ErrTruncated) {
		t.Fatalf("got %v", err)
	}
}

func TestEmitErrorStopsTheStream(t *testing.T) {
	stop := errors.New("stop")
	err := DecodeStream(bytes.NewReader(loadNamed(t, "size70000-level3")), func([]byte) error { return stop })
	if err != stop {
		t.Fatalf("got %v", err)
	}
}

func TestAMatchNineMebibytesBackIsDecoded(t *testing.T) {
	// The frame announces a 128 MiB window and holds a match 9 MiB back.
	for _, test := range loadFrames(t) {
		if test.name != "far-reference" {
			continue
		}
		got, err := DecodeAll(test.frame, 1<<30)
		if err != nil || len(got) != test.size || digestOf(got) != test.digest {
			t.Fatalf("got %d bytes, digest %s, err %v", len(got), digestOf(got), err)
		}
	}
}

func TestAWindowAboveTheLimitIsRefusedWithoutAllocatingIt(t *testing.T) {
	frame := loadNamed(t, "too-wide-window")
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := DecodeAll(frame, 1<<30)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrTooLarge) || errors.Is(err, ErrCorrupt) {
		t.Fatalf("got %v", err)
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown > 16<<20 {
		t.Fatalf("a refused frame allocated %d bytes", grown)
	}
}

func TestASmallFrameAnnouncingTheLargestWindowCostsAboutThatWindow(t *testing.T) {
	// The frame of too-wide-window with its window byte set to 2^27 bytes.
	frame := append([]byte{}, loadNamed(t, "too-wide-window")...)
	frame[5] = 17 << 3
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var total int
	err := DecodeStream(bytes.NewReader(frame), func(chunk []byte) error {
		total += len(chunk)
		return nil
	})
	runtime.ReadMemStats(&after)
	if err != nil || total != 5000 {
		t.Fatalf("decoded %d bytes, err %v", total, err)
	}
	grown := after.TotalAlloc - before.TotalAlloc
	if grown < MaxWindowSize || grown > MaxWindowSize+2<<20 {
		t.Fatalf("one decode allocated %d bytes, want the window of %d and under 2 MiB more", grown, MaxWindowSize)
	}
}

func TestADecodeStartsNoGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	frame := loadNamed(t, "size300000-level3")
	err := DecodeStream(bytes.NewReader(frame), func([]byte) error {
		if during := runtime.NumGoroutine(); during > before {
			t.Errorf("%d goroutines during the decode, %d before", during, before)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("%d goroutines after the decode, %d before", after, before)
	}
}

func TestADamagedBlockInFullInputIsNeverCalledTruncatedEvenFromAReaderThatReturnsEOFWithData(t *testing.T) {
	good := loadNamed(t, "size300000-level19")
	for place := 20; place < len(good); place += 1 + len(good)/400 {
		damaged := flip2(good, place, 0xFF)
		_, err := DecodeAll(damaged, 1<<30)
		if errors.Is(err, ErrTruncated) {
			t.Fatalf("byte %d damaged: got %v", place, err)
		}
		var out bytes.Buffer
		err = DecodeStream(&dataWithEOF{data: damaged}, func(chunk []byte) error {
			out.Write(chunk)
			return nil
		})
		if errors.Is(err, ErrTruncated) {
			t.Fatalf("byte %d damaged, EOF with the last data: got %v", place, err)
		}
	}
}

// dataWithEOF returns its bytes in one read together with io.EOF.
type dataWithEOF struct{ data []byte }

func (d *dataWithEOF) Read(buffer []byte) (int, error) {
	n := copy(buffer, d.data)
	d.data = d.data[n:]
	if len(d.data) > 0 {
		return n, nil
	}
	return n, io.EOF
}

func FuzzDecodeAll(fuzzer *testing.F) {
	for _, test := range loadFrames(fuzzer) {
		if len(test.frame) < 100000 {
			fuzzer.Add(test.frame)
		}
	}
	fuzzer.Fuzz(func(t *testing.T, data []byte) {
		_, err := DecodeAll(data, 1<<24)
		if err != nil && !errors.Is(err, ErrCorrupt) && !errors.Is(err, ErrTruncated) && !errors.Is(err, ErrTooLarge) {
			t.Fatalf("unclassified error %v", err)
		}
	})
}
