package textlines

import (
	"bufio"
	"context"
	"errors"
	"io"
)

// ReadLines calls yield with each line of r, LF-ended or last, without its
// LF, and its number from 1. A line longer than limit is not kept: yield
// gets it as nil with long set, and reading goes on after its LF, so one
// such line costs no more memory than limit. ctx is looked at for each
// line; a read error or its error ends the reading.
func ReadLines(ctx context.Context, r io.Reader, limit int,
	yield func(number int, line []byte, long bool),
) error {
	reader := bufio.NewReaderSize(r, 64<<10)
	var line []byte
	long := false
	for number := 1; ; {
		chunk, err := reader.ReadSlice('\n')
		if !long {
			line = append(line, chunk...)
			long = len(line) > limit+1 || len(line) > limit && line[len(line)-1] != '\n'
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil && err != io.EOF {
			return err
		}
		if len(line) > 0 || long {
			if err := ctx.Err(); err != nil {
				return err
			}
			if long {
				line = nil
			}
			trimmed := line
			if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
				trimmed = trimmed[:len(trimmed)-1]
			}
			yield(number, trimmed, long)
			number++
		}
		if err == io.EOF {
			return nil
		}
		line, long = line[:0], false
	}
}
