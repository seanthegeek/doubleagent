package analyze

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/rowfilter"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The files timeline writes into its output directory.
const (
	TimelineFile  = "timeline.jsonl"
	SessionsFile  = "sessions.jsonl"
	InventoryFile = "inventory.json"
)

// buildFilter returns the row filter of --since, --until, --keep-undated
// and --match. Each bound reads the clock once, as it is parsed.
func buildFilter(options TimelineOptions, now func() time.Time) (*rowfilter.RowFilter, error) {
	filter := &rowfilter.RowFilter{KeepUndated: options.KeepUndated}
	var err error
	if options.Since != "" {
		if filter.Since, err = rowfilter.ParseBound(options.Since, false, now()); err != nil {
			return nil, err
		}
	}
	if options.Until != "" {
		if filter.Until, err = rowfilter.ParseBound(options.Until, true, now()); err != nil {
			return nil, err
		}
	}
	if filter.Since != "" && filter.Until != "" && filter.Since > filter.Until {
		return nil, errors.New("--since " + filter.Since + " is after --until " + filter.Until)
	}
	filter.Patterns, err = rowfilter.CompilePatterns(options.Match, options.IgnoreCase)
	if err != nil {
		return nil, err
	}
	return filter, nil
}

// timelineRun is one run of timeline: its options, environment, input and
// parsers, and what the parse gave.
type timelineRun struct {
	options    TimelineOptions
	env        Environment
	collection *input.Collection
	registry   *parsers.Registry
	found      *scratchFinder
	output     string
	filter     *rowfilter.RowFilter
	rows       []timeline.Row
	counts     map[[2]string]int
	parsed     []string
	dropped    map[string]int
	problems   []string
	// problemSecrets holds, for each problem a parse gave, what that parse
	// identified; a problem past its end is the command's own.
	problemSecrets []*secrets.Found
	sessions       []timeline.SessionSummary
	repositories   map[string]projectGit
	detected       detection
	out            output
}

// collectOutput reads every string the run's output will hold for the
// credentials of the URLs in it (secrets.Recorder.Collect): the rows and
// sessions, which are all in memory, the problems, what detect finds, the
// match patterns, the output directory, and the project paths of
// collection.json and of project homes as the input spells them and as
// inventory.json names them. inventory.json and the summary are made of
// these.
func (r *timelineRun) collectOutput() {
	recorder := r.out.recorder
	if recorder == nil {
		return
	}
	for _, row := range r.rows {
		row.Collect(recorder)
	}
	for _, session := range r.sessions {
		session.Collect(recorder)
	}
	for _, text := range slices.Concat(r.problems, r.options.Match, []string{r.output},
		collectionProjectTexts(r.collection), collectedProjects(r.collection)) {
		recorder.Collect(text)
	}
	r.out.collect(newDetectionJSON(r.detected))
}

// runTimeline parses every artifact of the input into the timeline,
// writes timeline.jsonl, sessions.jsonl and inventory.json, and prints the
// summary. It exits 1 when no row was written.
func runTimeline(ctx context.Context, options TimelineOptions, env Environment, stdout,
	stderr io.Writer) int {
	// Made before anything is printed, so every message goes through it.
	out := output{secrets.NewRecorder(options.Secrets)}
	filter, err := buildFilter(options, env.Now)
	if err != nil {
		return out.printError(ctx, stderr, err.Error())
	}
	found, err := matcher()
	if err != nil {
		return out.printError(ctx, stderr, err.Error())
	}
	registry := env.Parsers()
	if unknown := unknownAgent(registry, options.Agents); unknown != "" {
		return out.printError(ctx, stderr, "--agent "+unknown+": no parser reads this agent")
	}
	// Cleaned once, so the directories made and those removed on failure
	// are the same ones, whatever .. the analyst wrote.
	options.Output = filepath.Clean(options.Output)
	created, code := prepareOutput(options, out, stderr)
	defer removeCreated(created)
	if code != ExitFound {
		return code
	}
	collection, workCreated, message := openInput(ctx, options.InputOptions, found, env, out.recorder)
	defer removeCreated(workCreated)
	if collection == nil {
		return out.printError(ctx, stderr, message)
	}
	defer closeInput(ctx, collection, options.KeepExtracted, out, stderr)
	run := &timelineRun{options: options, env: env, collection: collection,
		registry: registry, found: newScratchFinder(found), output: options.Output,
		filter: filter, out: out}
	if err := run.parse(ctx); err != nil {
		return out.printError(ctx, stderr, err.Error())
	}
	// Everything the output holds is read for URL credentials before any of
	// it is written, so a long one is replaced in every output string.
	run.repositories = projectRepositories(collection.Artifacts, &run.problems)
	run.detected = detectWith(collection, parserAgents(registry), run.repositories)
	run.collectOutput()
	if err := run.writeRows(); err != nil {
		return out.printError(ctx, stderr, err.Error())
	}
	return run.summarise(ctx, stdout, stderr)
}

// unknownAgent returns the first of agents that no parser of registry
// reads, or "".
func unknownAgent(registry *parsers.Registry, agents []string) string {
	known := parsers.ByAgent(registry.All())
	for _, agent := range agents {
		if len(known[agent]) == 0 {
			return agent
		}
	}
	return ""
}

// prepareOutput checks the output directory before the input is touched and
// makes it when it is missing. It returns the directories it made, deepest
// first, and ExitFound, or the exit code of the refusal it printed to stderr.
// A symbolic link is never followed: one is not a directory.
func prepareOutput(options TimelineOptions, out output, stderr io.Writer) ([]string, int) {
	output := options.Output
	refuse := func(message string) ([]string, int) {
		fmt.Fprintln(stderr, out.text(usageError("timeline", message).Error()))
		return nil, ExitError
	}
	info, err := osfile.Lstat(output)
	switch {
	case err == nil && !info.IsDir():
		return refuse("the output directory " + output + " is not a directory")
	case err == nil:
		entries, listErr := osfile.ReadDir(output)
		if listErr != nil {
			fmt.Fprint(stderr, "error: "+out.shown(reason.Describe(output, listErr))+"\n")
			return nil, ExitError
		}
		if len(entries) > 0 {
			return refuse("the output directory " + output + " is not empty")
		}
	case !errors.Is(err, fs.ErrNotExist):
		fmt.Fprint(stderr, "error: "+out.shown(reason.Describe(output, err))+"\n")
		return nil, ExitError
	}
	if insideDirectory(output, filepath.Clean(options.Input)) {
		return refuse("the output directory " + output + " must be outside the input")
	}
	created := missingDirectories(output)
	if err := osfile.MkdirAll(output, 0o777); err != nil {
		removeCreated(created)
		fmt.Fprint(stderr, "error: "+out.shown(reason.Describe(output, err))+"\n")
		return nil, ExitError
	}
	if !probeWrite(output) {
		removeCreated(created)
		return refuse("the output directory " + output + " cannot be written to")
	}
	return created, ExitFound
}

// missingDirectories returns path and each of its parents that does not
// exist yet, deepest first.
func missingDirectories(path string) []string {
	var missing []string
	for {
		if _, err := osfile.Lstat(path); err == nil {
			return missing
		}
		missing = append(missing, path)
		parent := filepath.Dir(path)
		if parent == path {
			return missing
		}
		path = parent
	}
}

// removeCreated removes the directories the run made, deepest first. A
// directory that holds anything, such as an output file, stays.
func removeCreated(directories []string) {
	for _, directory := range directories {
		if osfile.Remove(directory) != nil {
			return
		}
	}
}

// probeWrite reports whether a new file can be created in directory, by
// creating a file with a random name that must not exist yet and removing
// it.
func probeWrite(directory string) bool {
	random := make([]byte, 16)
	_, _ = rand.Read(random)
	probe := filepath.Join(directory, ".write-probe-"+hex.EncodeToString(random))
	file, err := createNew(probe, 0o600)
	if err != nil {
		return false
	}
	_ = file.Close()
	_ = osfile.Remove(probe)
	return true
}

// parse collects the rows of every artifact, drops the secondary rows a
// primary store holds, sorts and filters them, and summarises the sessions.
func (r *timelineRun) parse(ctx context.Context) error {
	result, err := r.registry.Collect(ctx, r.collection,
		parser.Options{IncludeThinking: r.options.IncludeThinking}, r.options.Agents)
	if err != nil {
		return err
	}
	r.problems, r.problemSecrets = result.Problems, result.ProblemSecrets
	r.rows, r.dropped = dedup.Deduplicate(result.Primary, result.Secondary, r.registry.Hooks())
	r.counts = countRows(r.rows)
	timeline.SortRows(r.rows)
	parsed := map[string]bool{}
	for key := range r.counts {
		parsed[key[0]] = true
	}
	r.parsed = sortedKeys(parsed)
	if r.filter.Active() {
		r.rows = r.filter.Apply(r.rows)
		r.counts = countRows(r.rows)
	}
	r.sessions = timeline.Summarise(r.rows, r.registry.Marks())
	return nil
}

// countRows returns the number of rows per agent and turn type.
func countRows(rows []timeline.Row) map[[2]string]int {
	counts := map[[2]string]int{}
	for _, row := range rows {
		counts[[2]string{row.Agent, row.TurnType}]++
	}
	return counts
}

// outputPath returns the path of one output file, as the summary prints it.
func (r *timelineRun) outputPath(name string) string {
	return filepath.Join(r.output, name)
}

// writeRows writes timeline.jsonl and sessions.jsonl, each row and session
// as it is written under the run's secrets setting (timeline.Row.Withheld).
func (r *timelineRun) writeRows() error {
	recorder := r.out.recorder
	err := writeOutput(r.outputPath(TimelineFile), func(w io.Writer) error {
		if recorder == nil {
			return timeline.WriteRows(w, r.rows)
		}
		withheld := make([]timeline.Row, 1)
		for _, row := range r.rows {
			withheld[0] = row.Withheld(recorder)
			if err := timeline.WriteRows(w, withheld); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeOutput(r.outputPath(SessionsFile), func(w io.Writer) error {
		withheld := make([]timeline.SessionSummary, len(r.sessions))
		for i, session := range r.sessions {
			withheld[i] = session.Withheld(recorder)
		}
		return timeline.WriteSessions(w, withheld)
	})
}

// createNew creates the file at path, failing when anything is there, a
// link included, so a file is never written through an entry another
// process placed in the output directory during the run.
func createNew(path string, perm fs.FileMode) (*os.File, error) {
	return osfile.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
}

// writeOutput creates the file at path, new, and writes it through write,
// buffered.
func writeOutput(path string, write func(w io.Writer) error) error {
	file, err := createNew(path, 0o666)
	if err != nil {
		return reason.Wrap(reason.Describe(path, err), err)
	}
	buffered := bufio.NewWriter(file)
	err = write(buffered)
	if err == nil {
		err = buffered.Flush()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return reason.Wrap(reason.Describe(path, err), err)
	}
	return nil
}

// summarise reads the projects' files, prints the summary, and writes
// inventory.json between its agents and rows lines' inputs, returning the
// exit code.
func (r *timelineRun) summarise(ctx context.Context, stdout, stderr io.Writer) int {
	detected := r.detected
	r.printHeader(stdout)
	agents := detectedAgents(detected)
	inventory := r.inventory(detected, agents)
	if err := writeOutput(r.outputPath(InventoryFile), func(w io.Writer) error {
		return writeJSON(w, inventory)
	}); err != nil {
		return r.out.printError(ctx, stderr, err.Error())
	}
	r.printTotals(stdout, agents)
	if len(r.rows) == 0 {
		return ExitNothing
	}
	return ExitFound
}

// printHeader prints the input, host, notes, problems, filters and the
// dropped duplicates.
func (r *timelineRun) printHeader(w io.Writer) {
	fmt.Fprint(w, "input:     "+r.out.shown(r.collection.Source)+" ("+r.collection.Kind+")\n")
	fmt.Fprint(w, "host:      "+r.out.shown(orUnknown(r.collection.Host))+"\n")
	for _, note := range r.collection.Notes {
		fmt.Fprint(w, "note:      "+r.out.shown(note)+"\n")
	}
	for _, problem := range r.withheldProblems() {
		fmt.Fprint(w, "problem:   "+shown(problem)+"\n")
	}
	filter := r.filter
	if filter.Since != "" || filter.Until != "" {
		kept := ""
		if filter.KeepUndated {
			kept = ", undated rows kept"
		}
		fmt.Fprint(w, "window:    "+orText(filter.Since, "(start)")+" to "+
			orText(filter.Until, "(end)")+kept+"\n")
	}
	if len(r.options.Match) > 0 {
		ignoring := ""
		if r.options.IgnoreCase {
			ignoring = " (ignore case)"
		}
		fmt.Fprint(w, "match:     "+r.out.shown(strings.Join(r.options.Match, " | "))+ignoring+"\n")
	}
	if filter.Active() {
		fmt.Fprintf(w, "filtered:  %d outside the window, %d without a timestamp, %d not matching\n",
			filter.DroppedWindow, filter.DroppedUndated, filter.DroppedMatch)
	}
	fmt.Fprint(w, "dropped:   "+r.out.shown(droppedSummary(r.dropped))+"\n")
}

// orText returns text, or fallback for an empty one.
func orText(text, fallback string) string {
	if text == "" {
		return fallback
	}
	return text
}

// droppedSummary returns the text of the dropped: line, naming the agents
// that had duplicates dropped.
func droppedSummary(dropped map[string]int) string {
	total := 0
	var named []string
	for _, agent := range sortedKeys(dropped) {
		total += dropped[agent]
		if dropped[agent] > 0 {
			named = append(named, fmt.Sprintf("%s %d", agent, dropped[agent]))
		}
	}
	text := fmt.Sprintf("%d duplicate rows from secondary sources", total)
	if total > 0 {
		text += " (" + strings.Join(named, ", ") + ")"
	}
	return text
}
