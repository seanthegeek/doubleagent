package zed

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/zstdframe"
)

// MaxDecompressed is the most bytes Budget.Decompress returns for one blob: far
// above any thread or session event an agent stores, and low enough that a
// small hostile frame cannot make the parser allocate without bound.
const MaxDecompressed = 256 << 20

// The decompression budget of one database or file, which bounds the bytes
// its blobs decompress to in all, since MaxDecompressed bounds only one
// blob: BudgetRatio times its size, and at least BudgetFloor.
const (
	// BudgetRatio is far above what zstd gives for the JSON text agents
	// store, which rarely compresses past 20 to 1, so only a frame built to
	// expand meets it.
	BudgetRatio = 64
	// BudgetFloor lets a small database hold four blobs of the largest
	// size one blob may have.
	BudgetFloor = 4 * MaxDecompressed
)

// errBlobTooLarge reports a blob that decodes to more than MaxDecompressed
// bytes.
var errBlobTooLarge = errors.New("zstd: the content is larger than the decoder allows")

// decompress returns the content of a zstd blob as Zed and OpenClaw write
// it, one frame with or without its content size in the header, decoded up
// to limit bytes, and how many bytes were decoded, the content returned or
// not. A frame cut off before its end gives the whole blocks decoded
// before the cut and no error. Past the limit the error wraps
// zstdframe.ErrTooLarge, and so it does for a frame announcing a window
// above limit, before the window is made, even when its content would fit:
// the window bounds how far back the frame may copy from, not its content,
// and a frame may announce more than it uses. Data that is not zstd or is
// corrupt is an error in the binary's own words.
func decompress(decoder *zstdframe.Decoder, blob []byte, limit int) ([]byte, int, error) {
	var out bytes.Buffer
	err := decoder.DecodeStream(bytes.NewReader(blob), uint64(limit), func(chunk []byte) error {
		if out.Len()+len(chunk) > limit {
			return zstdframe.ErrTooLarge
		}
		out.Write(chunk)
		return nil
	})
	switch {
	case err == nil, errors.Is(err, zstdframe.ErrTruncated):
		return out.Bytes(), out.Len(), nil
	case errors.Is(err, zstdframe.ErrTooLarge):
		return nil, limit, err
	}
	detail := strings.TrimPrefix(err.Error(), zstdframe.ErrCorrupt.Error()+": ")
	return nil, out.Len(), errors.New("zstd: corrupt data (" + detail + ")")
}

// Budget is what is left of the bytes the blobs of one database or file
// may decompress to in all. Once it is spent, every further blob is
// refused with an error that says so. Its blobs decode with the decoder of
// its SharedDecoder, so the window a frame announces is reserved once for
// all the budgets that share it, not once per blob, and is never larger
// than what is left of the budget.
type Budget struct {
	total, left int
	// shared holds the decoder; a Budget made without one makes its own.
	shared *SharedDecoder
}

// SharedDecoder is one zstd decoder for the budgets of a parse to share,
// so a window is reserved once for all of them. Its zero value is ready
// for use; the decoder is made for the first blob and kept, so the largest
// window a frame announced stays reserved while the SharedDecoder lives.
// It runs in the calling goroutine and holds only memory, so it needs no
// closing, and it is for one goroutine at a time.
type SharedDecoder struct {
	decoder *zstdframe.Decoder
}

// Budget returns the budget of a database or file of size bytes, decoding
// with the shared decoder: BudgetRatio times its size, and at least floor
// (BudgetFloor outside the tests).
func (s *SharedDecoder) Budget(size int64, floor int) *Budget {
	total := floor
	if size > int64(math.MaxInt/BudgetRatio) {
		total = math.MaxInt
	} else if size > 0 {
		total = max(total, int(size)*BudgetRatio)
	}
	return &Budget{total: total, left: total, shared: s}
}

// NewBudget returns the budget of a database or file of size bytes, as
// SharedDecoder.Budget does, with a decoder of its own.
func NewBudget(size int64, floor int) *Budget {
	return new(SharedDecoder).Budget(size, floor)
}

// Decompress returns the content of a zstd blob as decompress decodes it,
// refusing one that decodes to more than MaxDecompressed bytes, would take
// the budget past its total, or whose frame announces a window above what
// is left of it. The bytes a blob decodes, refused or not, are taken from
// the budget, and a blob refused for passing what is left of it spends the
// rest: near the end of a budget, a frame announcing a larger window than
// its content needs is refused, and so is every blob after it.
func (b *Budget) Decompress(blob []byte) ([]byte, error) {
	if b.shared == nil {
		b.shared = new(SharedDecoder)
	}
	if b.shared.decoder == nil {
		decoder, err := zstdframe.NewDecoder()
		if err != nil {
			return nil, err
		}
		b.shared.decoder = decoder
	}
	limit := min(MaxDecompressed, b.left)
	out, decoded, err := decompress(b.shared.decoder, blob, limit)
	b.left -= decoded
	if !errors.Is(err, zstdframe.ErrTooLarge) {
		return out, err
	}
	if limit == MaxDecompressed {
		return nil, errBlobTooLarge
	}
	b.left = 0
	return nil, fmt.Errorf("zstd: not decompressed: this database's blobs reached the limit of %d bytes "+
		"decompressed in all", b.total)
}
