package openclaw

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/zstdframe"
)

// windowFrame is a zstd frame of ten bytes that announces a window of
// 128 MiB, the largest the decoder accepts, and holds one raw block of the
// byte "{".
var windowFrame = string([]byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, (27 - 10) << 3, 0x09, 0x00, 0x00, '{'})

// windowRing is the most a parse's decoder reserves once: the largest
// window it accepts, two blocks of 128 KiB with a little room past each,
// and 128 KiB for the decoder itself and its chunk of output.
const windowRing = zstdframe.MaxWindowSize + 2*(128<<10+32) + 128<<10

// perFileAllowance is the most one ten-byte transcript may cost once the
// window is reserved: reading it, its rows and its problem rows.
const perFileAllowance = 32 << 10

// allocatedBy returns the bytes the heap gave out while run ran.
func allocatedBy(run func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestTheTranscriptsOfAParseReserveTheLargestWindowOnce(t *testing.T) {
	measured := map[int]uint64{}
	for _, n := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d transcripts", n), func(t *testing.T) {
			home := t.TempDir()
			for i := range n {
				writeHomeFile(t, home, ".openclaw/sessions/s"+strconv.Itoa(i)+".jsonl.zst", windowFrame)
			}
			subject := New()
			allocated := allocatedBy(func() {
				for i := range n {
					parseHomeFile(t, subject, home, ".openclaw/sessions/s"+strconv.Itoa(i)+".jsonl.zst")
				}
			})
			measured[n] = allocated
			t.Logf("%d transcripts of %d bytes allocated %d bytes", n, len(windowFrame), allocated)
			if limit := windowRing + uint64(n)*perFileAllowance; allocated > limit {
				t.Errorf("%d transcripts allocated %d bytes, more than one window and %d per file (%d)",
					n, allocated, perFileAllowance, limit)
			}
		})
	}
	if grown := measured[40] - measured[10]; grown > 30*perFileAllowance {
		t.Errorf("30 more transcripts allocated %d more bytes, more than %d each", grown, perFileAllowance)
	}
}
