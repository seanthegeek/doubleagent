package zstdframe

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"testing"
)

// decodeWith returns what decoder makes of data under maxWindow.
func decodeWith(decoder *Decoder, data []byte, maxWindow uint64) ([]byte, error) {
	var out bytes.Buffer
	err := decoder.DecodeStream(bytes.NewReader(data), maxWindow, func(chunk []byte) error {
		out.Write(chunk)
		return nil
	})
	return out.Bytes(), err
}

// newDecoder returns a Decoder closed when the test ends.
func newDecoder(t testing.TB) *Decoder {
	t.Helper()
	decoder, err := NewDecoder()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(decoder.Close)
	return decoder
}

// largestWindowFrame returns a frame of ten bytes announcing a window of
// MaxWindowSize and holding one raw byte.
func largestWindowFrame() []byte {
	return []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 17 << 3, 0x09, 0x00, 0x00, '{'}
}

func TestOneDecoderGivesEveryFrameWhatAFreshOneGives(t *testing.T) {
	decoder := newDecoder(t)
	damaged := flip2(loadNamed(t, "size300000-level19"), 40, 0xFF)
	for _, test := range loadFrames(t) {
		if test.name == "too-wide-window" {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			// A failed input before each one shows a failure leaves nothing
			// behind for the next input.
			if _, err := decodeWith(decoder, damaged, MaxWindowSize); err == nil {
				t.Fatal("the damaged frame decoded")
			}
			got, err := decodeWith(decoder, test.frame, MaxWindowSize)
			if err != nil || len(got) != test.size || digestOf(got) != test.digest {
				t.Fatalf("decoded %d bytes with digest %s, %v; want %d bytes with %s",
					len(got), digestOf(got), err, test.size, test.digest)
			}
		})
	}
}

func TestADecoderReservesTheLargestWindowOnceForManyInputs(t *testing.T) {
	frame := largestWindowFrame()
	// The window and two blocks of 128 KiB with a little room past each
	// (the vendored history.ensureBlockRing), and the decoder itself.
	ring := uint64(MaxWindowSize + 2*(128<<10+32) + 128<<10)
	for _, n := range []int{200, 800} {
		t.Run(fmt.Sprintf("%d inputs", n), func(t *testing.T) {
			decoder := newDecoder(t)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for range n {
				if got, err := decodeWith(decoder, frame, MaxWindowSize); err != nil || string(got) != "{" {
					t.Fatalf("got %q, %v", got, err)
				}
			}
			runtime.ReadMemStats(&after)
			// Each input of ten bytes may cost a few hundred bytes of state
			// and output once the window is made.
			if grown, limit := after.TotalAlloc-before.TotalAlloc, ring+uint64(n)*512; grown > limit {
				t.Errorf("%d inputs allocated %d bytes, more than one window and 512 each (%d)", n, grown, limit)
			}
		})
	}
}

func TestAFrameAboveTheCallersWindowIsTooLarge(t *testing.T) {
	frame := largestWindowFrame()
	frame[5] = 10 << 3 // 1 MiB
	tests := []struct {
		name      string
		maxWindow uint64
		want      error
	}{
		{"a window at the limit decodes", 1 << 20, nil},
		{"a window one byte past the limit is refused", 1<<20 - 1, ErrTooLarge},
		{"a limit below 1 KiB is taken as 1 KiB", 0, ErrTooLarge},
		{"a limit above MaxWindowSize is taken as MaxWindowSize", 1 << 40, nil},
	}
	decoder := newDecoder(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeWith(decoder, frame, test.maxWindow)
			if !errors.Is(err, test.want) || (test.want == nil && string(got) != "{") {
				t.Errorf("got %q, %v; want %v", got, err, test.want)
			}
		})
	}
	if _, err := decodeWith(decoder, largestWindowFrame(), 1<<40); err != nil {
		t.Errorf("the largest window under a limit above it: %v", err)
	}
	wider := largestWindowFrame()
	wider[5] = 18 << 3
	if _, err := decodeWith(decoder, wider, 1<<40); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a window above MaxWindowSize under a limit above it: %v", err)
	}
}

func TestALaterFrameOfAnInputIsHeldToTheWindowToo(t *testing.T) {
	small := largestWindowFrame()
	small[5] = 0 // 1 KiB
	wide := largestWindowFrame()
	wide[5] = 10 << 3 // 1 MiB
	got, err := decodeWith(newDecoder(t), append(small, wide...), 1<<10)
	if !errors.Is(err, ErrTooLarge) || string(got) != "{" {
		t.Errorf("got %q, %v; want the first frame's byte and ErrTooLarge", got, err)
	}
}

func TestAReusedDecoderStartsNoGoroutines(t *testing.T) {
	decoder := newDecoder(t)
	before := runtime.NumGoroutine()
	for range 3 {
		if _, err := decodeWith(decoder, loadNamed(t, "size300000-level3"), MaxWindowSize); err != nil {
			t.Fatal(err)
		}
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("%d goroutines after the decodes, %d before", after, before)
	}
}

func FuzzDecoderReuse(fuzzer *testing.F) {
	for _, test := range loadFrames(fuzzer) {
		if len(test.frame) < 100000 {
			fuzzer.Add(test.frame)
		}
	}
	seed := loadNamed(fuzzer, "size1000-level3")
	fuzzer.Fuzz(func(t *testing.T, data []byte) {
		decoder := newDecoder(t)
		first, firstErr := decodeWith(decoder, seed, MaxWindowSize)
		got, err := decodeWith(decoder, data, 1<<24)
		again, againErr := decodeWith(decoder, seed, MaxWindowSize)
		if firstErr != nil || againErr != nil || !bytes.Equal(first, again) {
			t.Fatalf("the seed decoded to %d bytes, %v, then after the input %d bytes, %v",
				len(first), firstErr, len(again), againErr)
		}
		var fresh bytes.Buffer
		freshErr := DecodeStream(bytes.NewReader(data), func(chunk []byte) error {
			if fresh.Len()+len(chunk) > 1<<24 {
				return ErrTooLarge
			}
			fresh.Write(chunk)
			return nil
		})
		if (err == nil) != (freshErr == nil) || (err == nil && !bytes.Equal(got, fresh.Bytes())) {
			t.Fatalf("a reused decoder gave %d bytes, %v; a fresh one %d bytes, %v",
				len(got), err, fresh.Len(), freshErr)
		}
	})
}
