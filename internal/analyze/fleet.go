package analyze

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// DefaultFleetName is the CSV fleet writes when -o is not given, in the
// current directory.
const DefaultFleetName = "fleet-inventory.csv"

// FleetColumns returns the columns of the fleet CSV, in order: the host
// line's run fields, then the agent line's fields.
func FleetColumns() []string {
	return append(hostFields(), agentFields()...)
}

// hostFields are the fields of a host line the CSV keeps.
func hostFields() []string {
	return []string{"host", "collector", "mode", "at"}
}

// agentFields are the fields of an agent line the CSV keeps.
func agentFields() []string {
	return []string{"user", "agent", "files", "bytes", "first", "last", "projects", "evidence"}
}

// fleetError is a fleet input that cannot be used, with the message the
// command prints.
type fleetError struct {
	message string
}

// Error returns the message.
func (e *fleetError) Error() string { return e.message }

// fleetInputs returns each input that is a file, and the regular files of
// each input that is a directory, sorted by name and not read recursively.
// An input that is a link, or neither a file nor a directory, is refused.
func fleetInputs(inputs []string) ([]string, error) {
	var files []string
	for _, given := range inputs {
		path := given
		info, err := osfile.Lstat(path)
		switch {
		case err != nil:
			return nil, &fleetError{reason.Describe(path, err)}
		case info.Mode().Type() == 0:
			files = append(files, path)
		case info.Mode()&fs.ModeSymlink != 0:
			return nil, &fleetError{path + " is a symlink; give its target"}
		case info.IsDir():
			listed, err := directoryFiles(path)
			if err != nil {
				return nil, err
			}
			files = append(files, listed...)
		default:
			return nil, &fleetError{path + " is neither a regular file nor a directory"}
		}
	}
	return files, nil
}

// directoryFiles returns the regular files directly in directory, sorted
// by name; a link is not a regular file.
func directoryFiles(directory string) ([]string, error) {
	entries, err := parser.ReadDir(directory)
	if err != nil {
		return nil, &fleetError{reason.Describe(directory, err)}
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	files := make([]string, len(names))
	for i, name := range names {
		files[i] = filepath.Join(directory, name)
	}
	return files, nil
}

// fleetRows returns the CSV rows of one saved inventory output and how many
// of its non-blank lines are not inventory records. A host line opens a run
// whose agent lines follow it; a host without agent lines gets a row of its
// own, and an agent line before any host line keeps only its host field.
func fleetRows(text string) ([][]string, int) {
	var rows [][]string
	junk := 0
	var host []string
	hostHasAgents := false
	closeHost := func() {
		if host != nil && !hostHasAgents {
			rows = append(rows, append(slices.Clone(host), make([]string, len(agentFields()))...))
		}
	}
	for _, line := range inputLines(text) {
		record, ok := inventoryRecord(line)
		switch {
		case !ok:
			if strings.TrimSpace(line) != "" {
				junk++
			}
		case record.Get("type").StringValue() == "host":
			closeHost()
			host, hostHasAgents = cells(record, hostFields()), false
		default:
			head := []string{cell("host", record.Get("host")), "", "", ""}
			if host != nil {
				head, hostHasAgents = slices.Clone(host), true
			}
			rows = append(rows, append(head, cells(record, agentFields())...))
		}
	}
	closeHost()
	return rows, junk
}

// inputLines returns the lines of text, each ending at LF, CR LF or a lone
// CR, without its end. A saved console output can hold a progress line
// ended by a lone CR before a record, so the record still starts a line.
// A line may be as long as the text.
func inputLines(text string) []string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(nil, max(len(text)+1, bufio.MaxScanTokenSize))
	scanner.Split(scanLineEnds)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// scanLineEnds is a bufio.SplitFunc for inputLines: a line ends at LF, at CR
// LF or at a CR not followed by LF; the last line may have no end.
func scanLineEnds(data []byte, atEOF bool) (int, []byte, error) {
	end := bytes.IndexAny(data, "\r\n")
	switch {
	case end < 0 && atEOF && len(data) > 0:
		return len(data), data, nil
	case end < 0:
		return 0, nil, nil
	case data[end] == '\n':
		return end + 1, data[:end], nil
	case end+1 < len(data) && data[end+1] == '\n':
		return end + 2, data[:end], nil
	case end+1 < len(data) || atEOF:
		return end + 1, data[:end], nil
	}
	return 0, nil, nil
}

// inventoryRecord returns the host or agent record a line holds: a JSON
// object whose type is "host" or "agent", once surrounding whitespace and
// leading byte-order marks are removed. Anything else is not a record.
func inventoryRecord(line string) (jsonvalue.Value, bool) {
	line = strings.TrimLeft(strings.TrimSpace(line), "\uFEFF")
	if !strings.HasPrefix(line, "{") {
		return jsonvalue.Value{}, false
	}
	record, err := jsonvalue.DecodeString(line)
	if err != nil || !record.IsObject() {
		return jsonvalue.Value{}, false
	}
	kind := record.Get("type")
	if !kind.IsString() || kind.StringValue() != "host" && kind.StringValue() != "agent" {
		return jsonvalue.Value{}, false
	}
	return record, true
}

// cells returns the CSV cells of a record's fields.
func cells(record jsonvalue.Value, fields []string) []string {
	out := make([]string, len(fields))
	for i, field := range fields {
		out[i] = cell(field, record.Get(field))
	}
	return out
}

// cell returns one field's CSV cell: "" for a missing or null value, a
// time field as a timeline timestamp, anything else as its row text (Text).
func cell(field string, value jsonvalue.Value) string {
	switch {
	case value.IsNull():
		return ""
	case field == "at" || field == "first" || field == "last":
		return timeutil.FromValue(value)
	}
	return value.Text()
}

// agentTotal is one line of the rollup: an agent, the hosts and the users
// (host and user pairs) it was found for, and the latest last time.
type agentTotal struct {
	agent        string
	hosts, users int
	latest       string
}

// rollup returns one total per agent of the rows, sorted by hosts, then
// users, both descending, then by agent.
func rollup(rows [][]string) []agentTotal {
	const hostIndex, userIndex, agentIndex, lastIndex = 0, 4, 5, 9
	hosts := map[string]map[string]bool{}
	users := map[string]map[[2]string]bool{}
	latest := map[string]string{}
	for _, row := range rows {
		agent := row[agentIndex]
		if agent == "" {
			continue
		}
		if hosts[agent] == nil {
			hosts[agent], users[agent] = map[string]bool{}, map[[2]string]bool{}
		}
		hosts[agent][row[hostIndex]] = true
		users[agent][[2]string{row[hostIndex], row[userIndex]}] = true
		latest[agent] = max(latest[agent], row[lastIndex])
	}
	totals := make([]agentTotal, 0, len(hosts))
	for agent := range hosts {
		totals = append(totals, agentTotal{agent, len(hosts[agent]), len(users[agent]), latest[agent]})
	}
	slices.SortFunc(totals, func(left, right agentTotal) int {
		return cmp.Or(cmp.Compare(right.hosts, left.hosts), cmp.Compare(right.users, left.users),
			strings.Compare(left.agent, right.agent))
	})
	return totals
}

// runFleet merges the inventory files into the fleet CSV and prints the
// rows written and the rollup per agent.
func runFleet(options FleetOptions, stdout, stderr io.Writer) int {
	files, err := fleetInputs(options.Inputs)
	if err != nil {
		fmt.Fprint(stderr, "error: "+err.Error()+"\n")
		return ExitError
	}
	var rows [][]string
	for _, path := range files {
		data, err := parser.ReadFile(path)
		if err != nil {
			fmt.Fprint(stderr, "error: "+reason.Describe(path, err)+"\n")
			return ExitError
		}
		text := strings.TrimPrefix(textlines.ValidUTF8(data), "\uFEFF")
		fileRows, junk := fleetRows(text)
		if junk > 0 {
			fmt.Fprintf(stderr, "skipped %d non-inventory %s in %s\n", junk, plural(junk, "line"), path)
		}
		rows = append(rows, fileRows...)
	}
	output := options.Output
	if err := writeFleetCSV(output, rows); err != nil {
		fmt.Fprint(stderr, "error: "+reason.Describe(output, err)+"\n")
		return ExitError
	}
	printFleet(stdout, rows, output)
	if len(rows) == 0 {
		return ExitNothing
	}
	return ExitFound
}

// plural returns noun, with an s unless count is one.
func plural(count int, noun string) string {
	if count == 1 {
		return noun
	}
	return noun + "s"
}

// writeFleetCSV writes the header and rows to path through encoding/csv
// with its defaults (fields quoted where needed, lines ending in LF),
// creating its parent directory.
func writeFleetCSV(path string, rows [][]string) error {
	if parent := pathParent(path); parent != "." {
		if err := osfile.MkdirAll(parent, 0o777); err != nil {
			return err
		}
	}
	var out bytes.Buffer
	if err := csv.NewWriter(&out).WriteAll(append([][]string{FleetColumns()}, rows...)); err != nil {
		return err
	}
	return osfile.WriteFile(path, out.Bytes(), 0o666)
}

// printFleet prints how many rows from how many hosts were written, and the
// rollup per agent when there is one.
func printFleet(w io.Writer, rows [][]string, output string) {
	hosts := map[string]bool{}
	for _, row := range rows {
		hosts[row[0]] = true
	}
	fmt.Fprintf(w, "rows:   %d from %d %s in %s\n", len(rows), len(hosts), plural(len(hosts), "host"),
		output)
	totals := rollup(rows)
	if len(totals) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%-22s %6s %6s  %s\n", "agent", "hosts", "users", "latest")
	for _, total := range totals {
		fmt.Fprintf(w, "%-22s %6d %6d  %s\n", total.agent, total.hosts,
			total.users, total.latest)
	}
}
