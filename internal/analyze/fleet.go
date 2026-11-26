package analyze

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// DefaultFleetName is the CSV fleet writes when -o is not given, in the
// current directory.
const DefaultFleetName = "fleet-inventory.csv"

// FleetColumns returns the columns of the fleet CSV: the host line's fields
// the CSV keeps (hostFields), then the agent line's (agentFields).
func FleetColumns() []string {
	return append(hostFields(), agentFields()...)
}

// hostFields lists the host line's columns.
func hostFields() []string {
	return []string{"host", "collector", "mode", "at"}
}

// agentFields lists the agent line's columns.
func agentFields() []string {
	return []string{"user", "agent", "files", "bytes", "first", "last", "projects", "evidence"}
}

// fleetInputs returns each input that is a file, and the regular files of
// each input that is a directory, sorted by name and not read recursively,
// with the directories listed. An input that is a link, or neither a file
// nor a directory, is refused.
func fleetInputs(inputs []string) ([]string, []inputDirectory, error) {
	var files []string
	var directories []inputDirectory
	for _, given := range inputs {
		path := given
		info, err := osfile.Lstat(path)
		switch {
		case err != nil:
			return nil, nil, reason.Wrap(reason.Describe(path, err), err)
		case info.Mode().Type() == 0:
			files = append(files, path)
		case info.Mode()&fs.ModeSymlink != 0:
			return nil, nil, errors.New(path + " is a symlink; give its target")
		case info.IsDir():
			listed, err := directoryFiles(path)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, listed...)
			directories = append(directories, inputDirectory{path, info})
		default:
			return nil, nil, errors.New(path + " is neither a regular file nor a directory")
		}
	}
	return files, directories, nil
}

// inputDirectory is an input directory of fleet: its path as given and the
// entry its lstat found, which the output is compared with.
type inputDirectory struct {
	path string
	info fs.FileInfo
}

// directoryFiles returns the regular files directly in directory, sorted
// by name, each joined to directory as the system finds it (belowGiven).
func directoryFiles(directory string) ([]string, error) {
	entries, err := parserkit.ReadDir(directory)
	if err != nil {
		return nil, reason.Wrap(reason.Describe(directory, err), err)
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
		files[i] = belowGiven(directory, name)
	}
	return files, nil
}

// fleetCell is one cell of a fleet row: its text as the record holds it,
// which the rollup counts and the terminal shows, and whether it is a JSON
// number, which the CSV writes as it is (csvText).
type fleetCell struct {
	text   string
	number bool
}

// csvText returns the cell as the CSV writes it: a number as written, other
// text through spreadsheetText.
func (c fleetCell) csvText() string {
	if c.number {
		return c.text
	}
	return spreadsheetText(c.text)
}

// fleetRows returns the rows of one saved inventory output and how many
// of its non-blank lines are not inventory records. A host line opens a run
// whose agent lines follow it; a host without agent lines gets a row of its
// own, and an agent line before any host line keeps only its host field.
func fleetRows(text string) ([][]fleetCell, int) {
	var rows [][]fleetCell
	junk := 0
	var host []fleetCell
	hostHasAgents := false
	closeHost := func() {
		if host != nil && !hostHasAgents {
			rows = append(rows, append(slices.Clone(host), make([]fleetCell, len(agentFields()))...))
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
			head := []fleetCell{cell("host", record.Get("host")), {}, {}, {}}
			if host != nil {
				head, hostHasAgents = slices.Clone(host), true
			}
			rows = append(rows, append(head, cells(record, agentFields())...))
		}
	}
	closeHost()
	return rows, junk
}

// inputLines returns the non-empty lines of text, split at every CR and LF:
// a saved console output can hold a progress line ended by a lone CR before
// a record, so the record still starts a line.
func inputLines(text string) []string {
	return strings.FieldsFunc(text, func(character rune) bool { return character == '\r' || character == '\n' })
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

// cells returns the cells of a record's fields.
func cells(record jsonvalue.Value, fields []string) []fleetCell {
	out := make([]fleetCell, len(fields))
	for i, field := range fields {
		out[i] = cell(field, record.Get(field))
	}
	return out
}

// cell returns one field's cell: "" for a missing or null value, a time
// field as a timeline timestamp, a number as written, anything else as its
// row text (Text).
func cell(field string, value jsonvalue.Value) fleetCell {
	switch {
	case value.IsNull():
		return fleetCell{}
	case field == "at" || field == "first" || field == "last":
		return fleetCell{text: timeutil.FromValue(value)}
	case value.IsNumber():
		return fleetCell{text: value.Text(), number: true}
	}
	return fleetCell{text: value.Text()}
}

// spreadsheetText returns a cell's text with a single quote in front when
// its first character is =, +, -, @, a tab or a carriage return, which a
// spreadsheet takes as the start of a formula (OWASP's "CSV Injection"
// lists them), so a cell from a hostile host shows as text and runs
// nothing. A look-alike such as the full-width equals sign starts none.
func spreadsheetText(text string) string {
	if text != "" && strings.ContainsRune("=+-@\t\r", rune(text[0])) {
		return "'" + text
	}
	return text
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
func rollup(rows [][]fleetCell) []agentTotal {
	const hostIndex, userIndex, agentIndex, lastIndex = 0, 4, 5, 9
	hosts := map[string]map[string]bool{}
	users := map[string]map[[2]string]bool{}
	latest := map[string]string{}
	for _, row := range rows {
		agent, host := row[agentIndex].text, row[hostIndex].text
		if agent == "" {
			continue
		}
		if hosts[agent] == nil {
			hosts[agent], users[agent] = map[string]bool{}, map[[2]string]bool{}
		}
		hosts[agent][host] = true
		users[agent][[2]string{host, row[userIndex].text}] = true
		latest[agent] = max(latest[agent], row[lastIndex].text)
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
// rows written and the rollup per agent. It stops, writing nothing, once
// ctx is cancelled, looked at before each input file.
func runFleet(ctx context.Context, options FleetOptions, stdout, stderr io.Writer) int {
	files, directories, err := fleetInputs(options.Inputs)
	if err != nil {
		fmt.Fprint(stderr, "error: "+runlog.Printable(err.Error())+"\n")
		return ExitError
	}
	if message := fleetOutputRefusal(options.Output, directories, files); message != "" {
		fmt.Fprintln(stderr, usageError("fleet", message))
		return ExitError
	}
	var rows [][]fleetCell
	for _, path := range files {
		if ctx.Err() != nil {
			return printError(ctx, stderr, "")
		}
		data, err := parserkit.ReadFile(path)
		if err != nil {
			fmt.Fprint(stderr, "error: "+runlog.Printable(reason.Describe(path, err))+"\n")
			return ExitError
		}
		text := strings.TrimPrefix(textlines.ValidUTF8(data), "\uFEFF")
		fileRows, junk := fleetRows(text)
		if junk > 0 {
			fmt.Fprintf(stderr, "skipped %d non-inventory %s in %s\n", junk, plural(junk, "line"), runlog.Printable(path))
		}
		rows = append(rows, fileRows...)
	}
	output := options.Output
	if err := writeFleetCSV(output, rows); err != nil {
		fmt.Fprint(stderr, "error: "+runlog.Printable(reason.Describe(output, err))+"\n")
		return ExitError
	}
	printFleet(stdout, rows, output)
	if len(rows) == 0 {
		return ExitNothing
	}
	return ExitFound
}

// fleetOutputRefusal returns why the CSV may not be written at output, or
// "": output is a link (never written through), the same file as an input
// file, given or found in an input directory, inside an input directory,
// or where it cannot be told whether it is (insideDirectory).
func fleetOutputRefusal(output string, directories []inputDirectory, files []string) string {
	if parserkit.IsLink(output) {
		return "the output " + output + " is a symbolic link; give its target"
	}
	for _, file := range files {
		if sameFile(output, file) {
			return "the output " + output + " is the input file " + file
		}
	}
	for _, directory := range directories {
		switch inside, err := insideDirectory(output, directory.info); {
		case err != nil:
			return "cannot tell whether the output " + output + " is outside the input directory " +
				directory.path + ": " + reason.Describe(output, err)
		case inside:
			return "the output " + output + " must be outside the input directory " + directory.path
		}
	}
	return ""
}

// plural returns noun, with an s unless count is one.
func plural(count int, noun string) string {
	if count == 1 {
		return noun
	}
	return noun + "s"
}

// writeFleetCSV writes the header and rows to path through encoding/csv,
// making the parent outputParent names, and replaces path with the CSV
// (replaceFile). When that fails, the directories it made are removed
// while empty, unless the parent as written has a . or .. element: the
// system resolves a .., so the names above it are then not its parents.
func writeFleetCSV(path string, rows [][]fleetCell) error {
	records := [][]string{FleetColumns()}
	for _, row := range rows {
		record := make([]string, len(row))
		for i, cell := range row {
			record[i] = cell.csvText()
		}
		records = append(records, record)
	}
	var out bytes.Buffer
	if err := csv.NewWriter(&out).WriteAll(records); err != nil {
		return err
	}
	parent := outputParent(path)
	var created []string
	if parent != "" {
		if !hasDotElement(parent) {
			created = missingDirectories(parent)
		}
		if err := osfile.MkdirAll(parent, 0o777); err != nil {
			removeCreated(created)
			return err
		}
	}
	if err := replaceFile(path, parent, out.Bytes()); err != nil {
		removeCreated(created)
		return err
	}
	return nil
}

// hasDotElement reports whether path, as written, has an element that is
// . or ..: the system resolves it where the name before it lies, which a
// step up by name (filepath.Dir) does not.
func hasDotElement(path string) bool {
	for _, element := range strings.Split(filepath.ToSlash(path), "/") {
		if element == "." || element == ".." {
			return true
		}
	}
	return false
}

// fleetTemporaryPrefix and fleetTemporarySuffix surround the random part
// of the name of fleet's temporary CSV, so a file a killed run leaves says
// what it is: a fleet CSV not finished, safe to delete.
const (
	fleetTemporaryPrefix = "doubleagent-fleet-"
	fleetTemporarySuffix = ".partial"
)

// fleetTemporaryName returns a new name for fleet's temporary file: the
// prefix, 16 random hexadecimal digits and the suffix.
func fleetTemporaryName() string {
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	return fleetTemporaryPrefix + hex.EncodeToString(random) + fleetTemporarySuffix
}

// replaceFile writes data to a new temporary file in parent, the directory
// outputParent gives ("" is the working directory), and renames it to path.
// Beside path, it is on path's file system and in the directories the
// outside-the-input check judged. On failure the temporary file is removed,
// path is left as it was, and the error names path.
func replaceFile(path, parent string, data []byte) error {
	temporary := fleetTemporaryName()
	if parent != "" {
		temporary = belowGiven(parent, temporary)
	}
	file, err := createNew(temporary, 0o666)
	if err != nil {
		return outputError(path, err)
	}
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = osfile.Rename(temporary, path)
	}
	if err != nil {
		_ = osfile.Remove(temporary)
		return outputError(path, err)
	}
	return nil
}

// outputError returns err as an error of writing path itself, its cause
// kept. An output that is a directory gives "is a directory", where the
// rename reports EEXIST (Unix) or ERROR_ACCESS_DENIED (Windows).
func outputError(path string, err error) error {
	var pathError *fs.PathError
	var linkError *os.LinkError
	switch {
	case errors.As(err, &pathError):
		err = pathError.Err
	case errors.As(err, &linkError):
		err = linkError.Err
	}
	if info, statErr := osfile.Lstat(path); statErr == nil && info.IsDir() {
		err = syscall.EISDIR
	}
	return &fs.PathError{Op: "write", Path: path, Err: err}
}

// printFleet prints how many rows from how many hosts were written, and the
// rollup per agent when there is one, the cells' text as the records hold
// it: the spreadsheet quote is the CSV's alone.
func printFleet(w io.Writer, rows [][]fleetCell, output string) {
	hosts := map[string]bool{}
	for _, row := range rows {
		hosts[row[0].text] = true
	}
	fmt.Fprintf(w, "rows:   %d from %d %s in %s\n", len(rows), len(hosts), plural(len(hosts), "host"),
		runlog.Printable(output))
	totals := rollup(rows)
	if len(totals) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%-22s %6s %6s  %s\n", "agent", "hosts", "users", "latest")
	for _, total := range totals {
		fmt.Fprintf(w, "%-22s %6d %6d  %s\n", runlog.Printable(total.agent), total.hosts,
			total.users, runlog.Printable(total.latest))
	}
}
