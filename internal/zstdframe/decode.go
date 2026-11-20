package zstdframe

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// MaxWindowSize is the largest window a frame may announce, in bytes: 128 MiB,
// the reference zstd library's default limit for a decoder
// (ZSTD_WINDOWLOG_LIMIT_DEFAULT, a window log of 27). The
// decoder reserves a buffer of this size when a frame announces it, whatever
// the frame later holds, so one decode takes at most this much plus a few
// hundred kilobytes of working space, and the output buffer DecodeAll builds.
// A frame announcing a larger window fails with ErrTooLarge.
const MaxWindowSize = 128 << 20

var (
	// ErrTruncated means the input ends inside a frame: the bytes read so far
	// are a valid start, and more were expected. A frame starts with a
	// four-byte magic number, which cannot be judged until it is whole, so
	// input that ends one to three bytes after the last frame is truncated
	// whatever those bytes hold.
	ErrTruncated = errors.New("zstd data ends inside a frame")
	// ErrCorrupt means the input is not valid zstd data, four or more bytes
	// after the last frame that are not a frame's magic number included.
	ErrCorrupt = errors.New("zstd data is corrupt")
	// ErrTooLarge means the decoded output is larger than the caller allows, or a
	// frame announces a window above MaxWindowSize.
	ErrTooLarge = errors.New("zstd output is larger than the limit")
)

// chunkSize is how much decoded output DecodeStream hands over at a time.
const chunkSize = 64 << 10

// trackedReader remembers the first error its source returned other than the
// end of input, so a failing source is not mistaken for corrupt data, and
// whether the decoder has asked for bytes beyond the end of the input. An end
// of input that arrives together with the last bytes is held back until the
// next read, so that "asked for more than exists" is told apart from "the
// input ended exactly here".
type trackedReader struct {
	source     io.Reader
	err        error
	pendingEOF bool
	reachedEnd bool
}

// Read reads from the source and records what came back.
func (t *trackedReader) Read(buffer []byte) (int, error) {
	if t.pendingEOF {
		t.reachedEnd = true
		return 0, io.EOF
	}
	n, err := t.source.Read(buffer)
	switch {
	case err == io.EOF && n > 0:
		t.pendingEOF = true
		return n, nil
	case err == io.EOF:
		t.reachedEnd = true
	case err != nil && t.err == nil:
		t.err = err
	}
	return n, err
}

// DecodeStream decodes the zstd frames of r, one after another, and passes the
// decoded bytes to emit in chunks. A chunk is only valid during the call.
// Output decoded before a failure has already been emitted, so a caller sees
// everything up to the damage. An empty stream decodes to nothing. The decode
// runs in the calling goroutine and takes at most MaxWindowSize bytes plus a
// little working space. The error wraps ErrTruncated when the input ends
// inside a frame, ErrCorrupt when it is not valid, ErrTooLarge when a frame
// announces a window above MaxWindowSize, and the reader's own error when
// reading fails; it is what emit returned when emit fails.
func DecodeStream(r io.Reader, emit func(chunk []byte) error) error {
	decoder, err := NewDecoder()
	if err != nil {
		return err
	}
	defer decoder.Close()
	return decoder.DecodeStream(r, MaxWindowSize, emit)
}

// Decoder decodes one zstd input after another as DecodeStream does, keeping
// its buffers from one input to the next. The window buffer, which the
// decoder makes as large as a frame's announced window before reading its
// blocks, is made again only for a frame announcing a larger window than
// any before it, so decoding many small inputs costs their output and one
// window, not a window each. A Decoder is for one goroutine at a time.
type Decoder struct {
	decoder *zstd.Decoder
	chunk   []byte
}

// NewDecoder returns a Decoder that runs in the calling goroutine.
func NewDecoder() (*Decoder, error) {
	decoder, err := zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxWindow(MaxWindowSize),
		zstd.WithDecoderLowmem(true))
	if err != nil {
		return nil, fmt.Errorf("start zstd decoder: %w", err)
	}
	return &Decoder{decoder: decoder, chunk: make([]byte, chunkSize)}, nil
}

// Close releases the decoder's buffers; the Decoder cannot be used after.
func (d *Decoder) Close() {
	d.decoder.Close()
}

// DecodeStream decodes the frames of r as the package's DecodeStream does,
// with one more limit: a frame announcing a window above maxWindow is
// refused with ErrTooLarge, before its window is made. maxWindow is taken
// as at least the 1 KiB every frame may announce and at most MaxWindowSize.
// The decoder checks the window at every frame's header, so a later frame
// of r is held to it too.
func (d *Decoder) DecodeStream(r io.Reader, maxWindow uint64, emit func(chunk []byte) error) error {
	tracked := &trackedReader{source: r}
	maxWindow = min(max(maxWindow, zstd.MinWindowSize), MaxWindowSize)
	if err := d.decoder.ResetWithOptions(tracked, zstd.WithDecoderMaxWindow(maxWindow)); err != nil {
		return fmt.Errorf("start zstd decoder: %w", err)
	}
	// A reset to no reader drops the decoder's hold on r and its bytes.
	defer d.decoder.Reset(nil)
	for {
		n, err := d.decoder.Read(d.chunk)
		if n > 0 {
			if emitErr := emit(d.chunk[:n]); emitErr != nil {
				return emitErr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return classify(err, tracked)
		}
	}
}

// classify sorts a decoder failure into the package's error values. The
// decoder reports io.ErrUnexpectedEOF both when the input ran out and when a
// block it had read in full is damaged, so that error counts as truncation
// only if the decoder really reached the end of the input.
func classify(err error, tracked *trackedReader) error {
	switch {
	case tracked.err != nil:
		return fmt.Errorf("read zstd input: %w", tracked.err)
	case errors.Is(err, zstd.ErrWindowSizeExceeded), errors.Is(err, zstd.ErrDecoderSizeExceeded):
		return fmt.Errorf("%w: %w", ErrTooLarge, err)
	case errors.Is(err, io.ErrUnexpectedEOF) && tracked.reachedEnd:
		return fmt.Errorf("%w: %w", ErrTruncated, err)
	}
	return fmt.Errorf("%w: %w", ErrCorrupt, err)
}

// DecodeAll decodes data, which holds one or more concatenated frames (and
// any skippable frames between them), and returns the decoded bytes. Empty
// input gives empty output. It fails with ErrTooLarge as soon as the output
// would pass limit bytes, so a small frame cannot grow into an unbounded
// allocation; the other errors are those of DecodeStream.
func DecodeAll(data []byte, limit int64) ([]byte, error) {
	var out bytes.Buffer
	err := DecodeStream(bytes.NewReader(data), func(chunk []byte) error {
		if int64(out.Len())+int64(len(chunk)) > limit {
			return ErrTooLarge
		}
		out.Write(chunk)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
