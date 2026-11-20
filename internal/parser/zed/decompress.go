package zed

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/zstdframe"
)

// MaxDecompressed is the most bytes Decompress returns for one blob: far
// above any thread or session event an agent stores, and low enough that a
// small hostile frame cannot make the parser allocate without bound.
const MaxDecompressed = 256 << 20

// The decompression budget of one database, which bounds the bytes its
// blobs decompress to in all, since MaxDecompressed bounds only one blob:
// BudgetRatio times the database's size, and at least BudgetFloor.
const (
	// BudgetRatio is far above what zstd gives for the JSON text agents
	// store, which rarely compresses past 20 to 1, so only a frame built to
	// expand meets it.
	BudgetRatio = 64
	// BudgetFloor lets a small database hold four blobs of the largest
	// size one blob may have.
	BudgetFloor = 4 * MaxDecompressed
)

// Decompress returns the content of a zstd blob as Zed and OpenClaw write
// it: one frame, with or without its content size in the header. A frame
// cut off before its end gives the whole blocks decoded before the cut and
// no error, so the caller reports the shortened text rather than the
// compression. Data that is not zstd, is corrupt, or decodes to more than
// MaxDecompressed bytes is an error in the binary's own words.
func Decompress(blob []byte) ([]byte, error) {
	decoder, err := zstdframe.NewDecoder()
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	out, _, err := decompress(decoder, blob, MaxDecompressed)
	if errors.Is(err, zstdframe.ErrTooLarge) {
		return nil, errBlobTooLarge
	}
	return out, err
}

// errBlobTooLarge reports a blob that decodes to more than MaxDecompressed
// bytes.
var errBlobTooLarge = errors.New("zstd: the content is larger than the decoder allows")

// decompress returns the content of a zstd blob decoded up to limit bytes
// and how many bytes were decoded, the content returned or not; past the
// limit the error wraps zstdframe.ErrTooLarge. A frame announcing a window
// above limit is refused the same way before its window is made: a window
// holds output already decoded, so a frame that needs a larger one would
// pass the limit anyway, and one that only announces it would make the
// decoder reserve memory its content never uses.
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

// Budget is what is left of the bytes the blobs of one database may
// decompress to in all. Once it is spent, every further blob is refused
// with an error that says so. Its blobs share one decoder, so the window a
// frame announces is reserved once for the database, not once per blob,
// and is never larger than what is left of the budget.
type Budget struct {
	total, left int
	// decoder is made for the first blob and kept for the others. It runs
	// in the calling goroutine and holds only memory, so a Budget needs no
	// closing.
	decoder *zstdframe.Decoder
}

// NewBudget returns the budget of a database of size bytes: BudgetRatio
// times its size, and at least floor (BudgetFloor outside the tests).
func NewBudget(size int64, floor int) *Budget {
	total := floor
	if size > int64(math.MaxInt/BudgetRatio) {
		total = math.MaxInt
	} else if size > 0 {
		total = max(total, int(size)*BudgetRatio)
	}
	return &Budget{total: total, left: total}
}

// Decompress decompresses a blob as Decompress does, also refusing one
// that would take the database past its budget or whose frame announces a
// window above what is left of it. The bytes a blob decodes, refused or
// not, are taken from the budget.
func (b *Budget) Decompress(blob []byte) ([]byte, error) {
	if b.decoder == nil {
		decoder, err := zstdframe.NewDecoder()
		if err != nil {
			return nil, err
		}
		b.decoder = decoder
	}
	limit := min(MaxDecompressed, b.left)
	out, decoded, err := decompress(b.decoder, blob, limit)
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
