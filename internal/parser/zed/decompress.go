package zed

import (
	"bytes"
	"errors"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/zstdframe"
)

// MaxDecompressed is the most bytes Decompress returns for one blob: far
// above any thread or session event an agent stores, and low enough that a
// small hostile frame cannot make the parser allocate without bound.
const MaxDecompressed = 256 << 20

// Decompress returns the content of a zstd blob as Zed and OpenClaw write
// it: one frame, with or without its content size in the header. A frame
// cut off before its end gives the whole blocks decoded before the cut and
// no error, so the caller reports the shortened text rather than the
// compression. Data that is not zstd, is corrupt, or decodes to more than
// MaxDecompressed bytes is an error in the binary's own words.
func Decompress(blob []byte) ([]byte, error) {
	var out bytes.Buffer
	err := zstdframe.DecodeStream(bytes.NewReader(blob), func(chunk []byte) error {
		if out.Len()+len(chunk) > MaxDecompressed {
			return zstdframe.ErrTooLarge
		}
		out.Write(chunk)
		return nil
	})
	switch {
	case err == nil, errors.Is(err, zstdframe.ErrTruncated):
		return out.Bytes(), nil
	case errors.Is(err, zstdframe.ErrTooLarge):
		return nil, errors.New("zstd: the content is larger than the decoder allows")
	}
	detail := strings.TrimPrefix(err.Error(), zstdframe.ErrCorrupt.Error()+": ")
	return nil, errors.New("zstd: corrupt data (" + detail + ")")
}
