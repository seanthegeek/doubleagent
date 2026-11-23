package zed

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/zstdframe"
)

// windowFrame returns a zstd frame of ten bytes that announces a window of
// 2^log bytes and holds one raw block of the byte "{".
func windowFrame(log byte) []byte {
	return []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, (log - 10) << 3, 0x09, 0x00, 0x00, '{'}
}

// windowRing is the most a database's decoder reserves once: the largest
// window it accepts and two blocks of 128 KiB with a little room past each
// (the vendored history.ensureBlockRing), and 128 KiB for the decoder itself
// and its chunk of output.
const windowRing = zstdframe.MaxWindowSize + 2*(128<<10+32) + 128<<10

// allocatedBy returns the bytes the heap gave out while run ran.
func allocatedBy(run func()) int64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return int64(after.TotalAlloc - before.TotalAlloc)
}

// perBlobAllowance is the most one ten-byte blob may cost once the window is
// reserved: its output, the decoder's per-input state and the Budget's
// copy of the output: about 240 bytes in all, whatever the window.
const perBlobAllowance = 512

func TestTheBlobsOfADatabaseReserveTheLargestWindowOnce(t *testing.T) {
	frame := windowFrame(27)
	measured := map[int]int64{}
	for _, n := range []int{200, 800} {
		t.Run(fmt.Sprintf("%d blobs", n), func(t *testing.T) {
			budget := NewBudget(int64(n*len(frame)), BudgetFloor)
			allocated := allocatedBy(func() {
				for range n {
					if out, err := budget.Decompress(frame); err != nil || string(out) != "{" {
						t.Fatalf("got %q, %v; want the one byte", out, err)
					}
				}
			})
			measured[n] = allocated
			t.Logf("%d blobs of %d bytes allocated %d bytes", n, len(frame), allocated)
			if limit := windowRing + int64(n)*perBlobAllowance; allocated > limit {
				t.Errorf("%d blobs of %d bytes allocated %d bytes, more than one window and %d per blob (%d)",
					n, len(frame), allocated, perBlobAllowance, limit)
			}
		})
	}
	if grown := measured[800] - measured[200]; grown > 600*perBlobAllowance {
		t.Errorf("600 more blobs allocated %d more bytes, more than %d each", grown, perBlobAllowance)
	}
}

func TestADecodeOfABlobAfterTheFirstMakesFewAllocations(t *testing.T) {
	frame := windowFrame(27)
	budget := NewBudget(0, BudgetFloor)
	allocations := testing.AllocsPerRun(100, func() {
		if out, err := budget.Decompress(frame); err != nil || string(out) != "{" {
			t.Fatalf("got %q, %v; want the one byte", out, err)
		}
	})
	if allocations > 10 {
		t.Errorf("one blob made %v allocations, want at most 10", allocations)
	}
}

func TestAFrameWhoseWindowIsAboveTheLimitIsRefused(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
		total int
		want  string
	}{
		{"a window of 2 MiB, as a streaming encoder writes, decodes", windowFrame(21), BudgetFloor, ""},
		{"the largest window the decoder accepts decodes", windowFrame(27), BudgetFloor, ""},
		{"a window above the decoder's limit is too large", windowFrame(28), BudgetFloor,
			errBlobTooLarge.Error()},
		{"a window above what is left of the budget passes it", windowFrame(20), 64 << 10,
			"zstd: not decompressed: this database's blobs reached the limit of 65536 bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			budget := &Budget{total: test.total, left: test.total}
			out, err := budget.Decompress(test.frame)
			switch {
			case test.want == "" && (err != nil || string(out) != "{"):
				t.Errorf("got %q, %v; want the one byte", out, err)
			case test.want != "" && (err == nil || !strings.HasPrefix(err.Error(), test.want)):
				t.Errorf("got %q, %v; want %q", out, err, test.want)
			}
		})
	}
}

// perRowAllowance is the most one thread of wide_window_<n>.db may cost
// once the window is reserved: its blob, its decoded JSON and its rows.
const perRowAllowance = 8 << 10

func TestADatabaseOfBlobsAnnouncingTheLargestWindowReservesItOnce(t *testing.T) {
	measured := map[int]int64{}
	for _, n := range []int{200, 800} {
		t.Run(fmt.Sprintf("%d threads", n), func(t *testing.T) {
			var rows int
			allocated := allocatedBy(func() {
				rows = len(parseThreadsFixture(t, New(), fmt.Sprintf("wide_window_%d.db", n)))
			})
			measured[n] = allocated
			t.Logf("%d threads allocated %d bytes", n, allocated)
			if rows != n {
				t.Fatalf("got %d rows, want a thread start for each of %d threads", rows, n)
			}
			if limit := windowRing + int64(n)*perRowAllowance; allocated > limit {
				t.Errorf("%d threads allocated %d bytes, more than one window and %d per thread (%d)",
					n, allocated, perRowAllowance, limit)
			}
		})
	}
	if grown := measured[800] - measured[200]; grown > 600*perRowAllowance {
		t.Errorf("600 more threads allocated %d more bytes, more than %d each", grown, perRowAllowance)
	}
}

func TestAWindowAboveTheDatabasesBudgetIsAProblemRow(t *testing.T) {
	subject := New()
	subject.budgetFloor = 0
	rows := parseThreadsFixture(t, subject, "wide_window_200.db")
	if len(rows) != 200 {
		t.Fatalf("got %d rows, want one for each of 200 threads", len(rows))
	}
	want := "zstd: not decompressed: this database's blobs reached the limit of "
	for i, row := range rows {
		prefix := fmt.Sprintf("parser: unparseable record %d (%s", i+1, want)
		if !strings.HasPrefix(row.Text, prefix) || !strings.Contains(row.Text, "; stored zstd blob as hex): 28b52ffd0088") {
			t.Fatalf("row %d is %q, want it to start %q and hold the blob as hex", i, row.Text, prefix)
		}
	}
}
