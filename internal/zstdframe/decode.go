package zstdframe

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// MaxWindowSize is the most earlier output the decoder keeps for back
// references. A frame may announce a larger window; the decoder reads it as
// long as no match reaches back further than this many bytes.
const MaxWindowSize = 8 << 20

var (
	// ErrTruncated means the input ends inside a frame: the bytes read so far
	// are a valid start, and more were expected.
	ErrTruncated = errors.New("zstd data ends inside a frame")
	// ErrCorrupt means the input is not valid zstd data.
	ErrCorrupt = errors.New("zstd data is corrupt")
	// ErrTooLarge means the decoded output is larger than the caller allows.
	ErrTooLarge = errors.New("zstd output is larger than the limit")
)

// chunkSize is how much decoded output DecodeStream hands over at a time.
const chunkSize = 64 << 10

// trackedReader counts the bytes its source returned and remembers the first
// error other than the end of input, so a failing source is not mistaken for
// corrupt data.
type trackedReader struct {
	source io.Reader
	total  int
	err    error
}

// Read reads from the source and records what came back.
func (t *trackedReader) Read(buffer []byte) (int, error) {
	n, err := t.source.Read(buffer)
	t.total += n
	if err != nil && err != io.EOF && t.err == nil {
		t.err = err
	}
	return n, err
}

// DecodeStream decodes the zstd frames of r, one after another, and passes the
// decoded bytes to emit in chunks. A chunk is only valid during the call.
// Output decoded before a failure has already been emitted, so a caller sees
// everything up to the damage. An empty stream decodes to nothing.
//
// The error wraps ErrTruncated when the input ends inside a frame, ErrCorrupt
// when it is not valid, the reader's own error when reading fails, and is
// whatever emit returned when emit fails.
func DecodeStream(r io.Reader, emit func(chunk []byte) error) error {
	tracked := &trackedReader{source: r}
	decoder := NewReader(tracked)
	chunk := make([]byte, chunkSize)
	for {
		n, err := decoder.Read(chunk)
		if n > 0 {
			if emitErr := emit(chunk[:n]); emitErr != nil {
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

// classify sorts a decoder failure into the package's error values. The copied
// reader insists on at least one frame; an input with no bytes at all is
// an empty stream here, not a failure.
func classify(err error, tracked *trackedReader) error {
	switch {
	case tracked.err != nil:
		return fmt.Errorf("read zstd input: %w", tracked.err)
	case errors.Is(err, io.ErrUnexpectedEOF) && tracked.total == 0:
		return nil
	case errors.Is(err, io.ErrUnexpectedEOF):
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
