package zstdframe

import (
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// MaxWindowSize is the largest window a frame may announce, in bytes: 128
// MiB, the reference zstd library's default limit for a decoder
// (ZSTD_WINDOWLOG_LIMIT_DEFAULT, a window log of 27). The decoder reserves
// a buffer of the announced size whatever the frame later holds, so one
// decode takes at most this much plus a few hundred kilobytes.
const MaxWindowSize = 128 << 20

var (
	// ErrTruncated means the input ends inside a frame. A frame's four-byte
	// magic number cannot be judged until it is whole, so input that ends one
	// to three bytes after the last frame is truncated whatever they hold.
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

// Decoder decodes one zstd input after another, keeping its buffers from
// one input to the next. The window buffer is made again only for a frame
// announcing a larger window than any before it, so decoding many small
// inputs costs their output and one window, not a window each. A Decoder is
// for one goroutine at a time.
type Decoder struct {
	decoder *zstd.Decoder
	chunk   []byte
}

// NewDecoder returns a Decoder that runs in the calling goroutine.
func NewDecoder() (*Decoder, error) {
	decoder, err := zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxWindow(MaxWindowSize))
	if err != nil {
		return nil, fmt.Errorf("start zstd decoder: %w", err)
	}
	return &Decoder{decoder: decoder, chunk: make([]byte, chunkSize)}, nil
}

// DecodeStream decodes the zstd frames of r, one after another, and passes
// the decoded bytes to emit in chunks, each valid only during the call, so
// a caller sees everything up to any damage. A frame announcing a window
// above maxWindow, taken as at least 1 KiB and at most MaxWindowSize, is
// refused with ErrTooLarge before its window is made. The error wraps
// ErrTruncated, ErrCorrupt or ErrTooLarge, or the reader's own error; it is
// what emit returned when emit fails.
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
