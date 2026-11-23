package catalogtest

import (
	"math/rand/v2"
	"slices"
	"strings"
)

// headers are the seven section headers of a catalog text. A header written
// here differently from the loader's would file its lines under another
// table, which the tests that compare the parsed tables would catch.
var headers = []string{
	"# home artifacts (agent|glob relative to home)",
	"# project artifacts (relative to each discovered project)",
	"# excluded unless --full",
	"# credential files (flagged secret, skipped with --no-secrets)",
	"# docker volumes (agent|volume name glob)",
	"# relocation variables (agent|VARIABLE|home-relative path it replaces)",
	"# startup files searched for them (scope|glob; scope home, system or project)",
}

// Shuffled returns text with its tables, and the lines of each table, in an
// order drawn from seed, the same order for the same seed.
func Shuffled(text []byte, seed uint64) []byte {
	random := rand.New(rand.NewPCG(seed, seed))
	return reorder(text, func(lines []string) {
		random.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
	})
}

// Reversed returns text with its tables, and the lines of each table, in
// reverse order.
func Reversed(text []byte) []byte {
	return reorder(text, slices.Reverse[[]string])
}

// reorder returns the table lines of text, each under its section header as
// the loader files it, with permute applied to the list of headers and to the
// lines of each table. A line before any header belongs to the home table,
// as it does for the loader; comments and blank lines are left out.
func reorder(text []byte, permute func([]string)) []byte {
	tables := map[string][]string{}
	section := headers[0]
	for line := range strings.SplitSeq(string(text), "\n") {
		switch {
		case strings.Trim(line, " \t\r\v\f") == "":
		case slices.Contains(headers, line):
			section = line
		case strings.HasPrefix(line, "#"):
		default:
			tables[section] = append(tables[section], line)
		}
	}
	order := slices.Clone(headers)
	permute(order)
	var out strings.Builder
	for _, header := range order {
		lines := tables[header]
		permute(lines)
		out.WriteString(header + "\n")
		for _, line := range lines {
			out.WriteString(line + "\n")
		}
	}
	return []byte(out.String())
}
