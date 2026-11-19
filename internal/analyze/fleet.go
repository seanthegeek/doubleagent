package analyze

import (
	"bufio"
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
	"slices"
	"strings"
	"syscall"

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
			return nil, nil, &fleetError{reason.Describe(path, err)}
		case info.Mode().Type() == 0:
			files = append(files, path)
		case info.Mode()&fs.ModeSymlink != 0:
			return nil, nil, &fleetError{path + " is a symlink; give its target"}
		case info.IsDir():
			listed, err := directoryFiles(path)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, listed...)
			directories = append(directories, inputDirectory{path, info})
		default:
			return nil, nil, &fleetError{path + " is neither a regular file nor a directory"}
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
// by name; a link is not a regular file. Each path is the name below
// directory as the system finds it (belowGiven), so it names a file of the
// directory that was listed.
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
// it starts with =, +, -, @, a tab or a carriage return, the characters a
// spreadsheet that opens the CSV takes as the start of a formula (OWASP's
// "CSV Injection" lists them), so the cell shows as text and runs nothing:
// the inventory comes from the examined host, which may be hostile. Only
// the first character counts, as in that list; a look-alike such as the
// full-width equals sign starts no formula and is left as it is. A JSON
// number is written as a number, never through here.
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
		fmt.Fprint(stderr, "error: "+shown(err.Error())+"\n")
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
		data, err := parser.ReadFile(path)
		if err != nil {
			fmt.Fprint(stderr, "error: "+shown(reason.Describe(path, err))+"\n")
			return ExitError
		}
		text := strings.TrimPrefix(textlines.ValidUTF8(data), "\uFEFF")
		fileRows, junk := fleetRows(text)
		if junk > 0 {
			fmt.Fprintf(stderr, "skipped %d non-inventory %s in %s\n", junk, plural(junk, "line"), shown(path))
		}
		rows = append(rows, fileRows...)
	}
	output := options.Output
	if err := writeFleetCSV(output, rows); err != nil {
		fmt.Fprint(stderr, "error: "+shown(reason.Describe(output, err))+"\n")
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
	if isLink(output) {
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

// writeFleetCSV writes the header and rows to path through encoding/csv
// with its defaults (fields quoted where needed, lines ending in LF),
// creating its parent directory, the one outputParent names, so the file
// goes where fleetOutputRefusal looked. The CSV is written to a new file
// beside path and moved over it (replaceFile), so an existing file at path
// is replaced, never written into: a second name of an evidence file (a
// hard link) passes the outside-the-input check, which judges where the
// name lies, and writing through it would change the evidence. A link at
// path was refused before the inputs were read; one placed after that look
// is replaced itself, its target untouched.
func writeFleetCSV(path string, rows [][]fleetCell) error {
	parent := outputParent(path)
	if parent != "" {
		if err := osfile.MkdirAll(parent, 0o777); err != nil {
			return err
		}
	}
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
	return replaceFile(path, parent, out.Bytes(), fleetTemporaryName)
}

// fleetTemporaryPrefix and fleetTemporarySuffix surround the random part
// of the name of the file fleet writes its CSV to before moving it over
// the output, so a file left behind by a run that was killed says what it
// is: a fleet CSV not finished, safe to delete.
const (
	fleetTemporaryPrefix = "doubleagent-fleet-"
	fleetTemporarySuffix = ".partial"
)

// fleetTemporaryAttempts is how many temporary names replaceFile tries.
// Each name holds 64 random bits, so a second name is needed only when a
// file left by an earlier run holds the first; a name taken that many
// times in a row means the directory refuses every new name.
const fleetTemporaryAttempts = 10

// errNoTemporaryName is replaceFile's error when every name it tried was
// taken.
var errNoTemporaryName = errors.New("every temporary name tried beside it was taken")

// fleetTemporaryName returns a new name for fleet's temporary file: the
// prefix, 16 random hexadecimal digits and the suffix.
func fleetTemporaryName() string {
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	return fleetTemporaryPrefix + hex.EncodeToString(random) + fleetTemporarySuffix
}

// replaceFile puts data at path as a new file: it creates a file named by
// temporaryName in parent, the directory the system puts path's last
// element in (outputParent; "" is the working directory), writes data there
// and renames it to path. Lying beside path, the temporary file is on its
// file system for the rename, and the outside-the-input check made on path
// holds for it: that check compares the directories a name lies in with
// the input, and both names lie in the same ones. A taken name is passed
// over, as createNew never opens an existing entry. On any failure the
// temporary file is removed and whatever was at path is left as it was;
// the error names path, as a write to it would.
func replaceFile(path, parent string, data []byte, temporaryName func() string) error {
	temporary, file, err := createTemporary(parent, temporaryName)
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

// createTemporary creates a new file in parent under a name from
// temporaryName, trying up to fleetTemporaryAttempts names while each is
// taken, and returns its path and the open file.
func createTemporary(parent string, temporaryName func() string) (string, *os.File, error) {
	for range fleetTemporaryAttempts {
		temporary := temporaryName()
		if parent != "" {
			temporary = belowGiven(parent, temporary)
		}
		file, err := createNew(temporary, 0o666)
		if !errors.Is(err, fs.ErrExist) {
			return temporary, file, err
		}
	}
	return "", nil, errNoTemporaryName
}

// outputError returns err as an error of writing path itself, its cause
// kept: the temporary file is the command's own detail. An output that is
// a directory gives "is a directory", what opening it for writing gives,
// where the rename reports EEXIST (os.Rename on Unix) or ERROR_ACCESS_DENIED
// (MoveFileEx on Windows).
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
		shown(output))
	totals := rollup(rows)
	if len(totals) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%-22s %6s %6s  %s\n", "agent", "hosts", "users", "latest")
	for _, total := range totals {
		fmt.Fprintf(w, "%-22s %6d %6d  %s\n", shown(total.agent), total.hosts,
			total.users, shown(total.latest))
	}
}
