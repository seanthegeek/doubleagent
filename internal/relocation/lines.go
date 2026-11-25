package relocation

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// eachLine calls visit with the number and text of every line of r, split at
// line feeds only: the line feed is dropped, a carriage return kept, a last
// line without a line feed still counts, and nothing follows a final one.
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

// nameSet returns names as a set.
func nameSet(names []string) map[string]bool {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	return wanted
}
