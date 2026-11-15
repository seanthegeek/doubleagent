package relocation

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// eachLine calls visit with the number and text of every line of r, split at
// line feeds alone as grep and awk split them: the line feed is dropped, a
// last line without one still counts, and nothing follows a final line feed.
func eachLine(r io.Reader, visit func(number int, text []byte)) error {
	reader := bufio.NewReader(r)
	for number := 1; ; number++ {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			visit(number, line)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read line %d: %w", number, err)
		}
	}
}

// markNulBytes rewrites a line as `tr '\000\001' '\001\002'` does before the
// sh collector's awk readers, which would end a record at a NUL byte: a NUL
// becomes 0x01, which then marks a NUL for the reader, and a 0x01 becomes 0x02.
func markNulBytes(line []byte) string {
	marked := make([]byte, len(line))
	for i, char := range line {
		switch char {
		case 0x00:
			marked[i] = 0x01
		case 0x01:
			marked[i] = 0x02
		default:
			marked[i] = char
		}
	}
	return string(marked)
}

// nameSet returns names as a set, the awk readers' `wanted` array.
func nameSet(names []string) map[string]bool {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	return wanted
}
