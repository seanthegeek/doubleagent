package openclaw

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
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
func allocatedBy(run func()) int64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return int64(after.TotalAlloc - before.TotalAlloc)
}

// coldArchive returns the path, below a home, of the i-th cold archive
// file: a compressed transcript the parser reads.
func coldArchive(i int) string {
	return fmt.Sprintf(".openclaw/sessions/cold/%064x.jsonl.zst", i)
}

func TestTheTranscriptsOfAParseReserveTheLargestWindowOnce(t *testing.T) {
	measured := map[int]int64{}
	for _, n := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d transcripts", n), func(t *testing.T) {
			home := t.TempDir()
			subject := New()
			for i := range n {
				if !subject.Wants(&input.Artifact{Rel: coldArchive(i)}) {
					t.Fatalf("the parser is not given %s", coldArchive(i))
				}
				writeHomeFile(t, home, coldArchive(i), windowFrame)
			}
			allocated := allocatedBy(func() {
				for i := range n {
					parseHomeFile(t, subject, home, coldArchive(i))
				}
			})
			measured[n] = allocated
			t.Logf("%d transcripts of %d bytes allocated %d bytes", n, len(windowFrame), allocated)
			if limit := windowRing + int64(n)*perFileAllowance; allocated > limit {
				t.Errorf("%d transcripts allocated %d bytes, more than one window and %d per file (%d)",
					n, allocated, perFileAllowance, limit)
			}
		})
	}
	if grown := measured[40] - measured[10]; grown > 30*perFileAllowance {
		t.Errorf("30 more transcripts allocated %d more bytes, more than %d each", grown, perFileAllowance)
	}
}
