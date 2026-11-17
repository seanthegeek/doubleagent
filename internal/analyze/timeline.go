package analyze

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
		return nil, fmt.Errorf("--since %s is after --until %s", filter.Since, filter.Until)
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
	sessions   []timeline.SessionSummary
}

// runTimeline parses every artifact of the input into the timeline,
// writes timeline.jsonl, sessions.jsonl and inventory.json, and prints the
// summary. It exits 1 when no row was written.
func runTimeline(ctx context.Context, options TimelineOptions, env Environment, stdout,
	stderr io.Writer) int {
	filter, err := buildFilter(options, env.Now)
	if err != nil {
		return printError(ctx, stderr, err.Error())
	}
	found, err := matcher()
	if err != nil {
		return printError(ctx, stderr, err.Error())
	}
	collection, message := openInput(options.InputOptions, found)
	if collection == nil {
		return printError(ctx, stderr, message)
	}
	defer closeInput(collection, options.KeepExtracted, stderr)
	run := &timelineRun{options: options, env: env, collection: collection,
		registry: env.Parsers(), found: newScratchFinder(found), output: pathText(options.Output),
		filter: filter}
	if err := osfile.MkdirAll(run.output, 0o777); err != nil {
		return printError(ctx, stderr, reason.Describe(run.output, err))
	}
	if err := run.parse(ctx); err != nil {
		return printError(ctx, stderr, err.Error())
	}
	if err := run.writeRows(); err != nil {
		return printError(ctx, stderr, err.Error())
	}
	return run.summarise(ctx, stdout, stderr)
}

// parse collects the rows of every artifact, drops the secondary rows a
// primary store holds, sorts and filters them, and summarises the sessions.
func (r *timelineRun) parse(ctx context.Context) error {
	result, err := r.registry.Collect(ctx, r.collection,
		parser.Options{IncludeThinking: r.options.IncludeThinking}, r.options.Agents)
	if err != nil {
		return err
	}
	r.problems = result.Problems
	r.rows, r.dropped = dedup.Deduplicate(result.Primary, result.Secondary, r.registry.Hooks())
	r.counts = countRows(r.rows)
	timeline.SortRows(r.rows)
	r.parsed = nil
	for key := range r.counts {
		if !slices.Contains(r.parsed, key[0]) {
			r.parsed = append(r.parsed, key[0])
		}
	}
	slices.Sort(r.parsed)
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
	return pathText(r.output + "/" + name)
}

// writeRows writes timeline.jsonl and sessions.jsonl.
func (r *timelineRun) writeRows() error {
	err := writeOutput(r.outputPath(TimelineFile), func(w io.Writer) error {
		return timeline.WriteRows(w, r.rows)
	})
	if err != nil {
		return err
	}
	return writeOutput(r.outputPath(SessionsFile), func(w io.Writer) error {
		return timeline.WriteSessions(w, r.sessions)
	})
}

// writeOutput creates or replaces the file at path and writes it through
// write, buffered.
func writeOutput(path string, write func(w io.Writer) error) error {
	file, err := osfile.Create(path)
	if err != nil {
		return fmt.Errorf("%s", reason.Describe(path, err))
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
		return fmt.Errorf("%s", reason.Describe(path, err))
	}
	return nil
}

// summarise reads the projects' files, prints the summary, and writes
// inventory.json between its agents and rows lines' inputs, returning the
// exit code.
func (r *timelineRun) summarise(ctx context.Context, stdout, stderr io.Writer) int {
	parsed := parserAgents(r.registry)
	repositories := projectRepositories(r.collection.Artifacts, &r.problems)
	detected := detectWith(r.collection, parsed, repositories)
	r.printHeader(stdout)
	agents := detectedAgents(detected)
	inventory := r.inventory(detected, agents)
	if err := writeOutput(r.outputPath(InventoryFile), func(w io.Writer) error {
		_, err := io.WriteString(w, inventory)
		return err
	}); err != nil {
		return printError(ctx, stderr, err.Error())
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
	fmt.Fprintf(w, "input:     %s (%s)\n", r.collection.Source, r.collection.Kind)
	fmt.Fprintf(w, "host:      %s\n", orUnknown(r.collection.Host))
	for _, note := range r.collection.Notes {
		fmt.Fprintf(w, "note:      %s\n", note)
	}
	for _, problem := range r.problems {
		fmt.Fprintf(w, "problem:   %s\n", problem)
	}
	filter := r.filter
	if filter.Since != "" || filter.Until != "" {
		kept := ""
		if filter.KeepUndated {
			kept = ", undated rows kept"
		}
		fmt.Fprintf(w, "window:    %s to %s%s\n", orText(filter.Since, "(start)"),
			orText(filter.Until, "(end)"), kept)
	}
	if len(r.options.Match) > 0 {
		ignoring := ""
		if r.options.IgnoreCase {
			ignoring = " (ignore case)"
		}
		fmt.Fprintf(w, "match:     %s%s\n", strings.Join(r.options.Match, " | "), ignoring)
	}
	if filter.Active() {
		fmt.Fprintf(w, "filtered:  %d outside the window, %d without a timestamp, %d not matching\n",
			filter.DroppedWindow, filter.DroppedUndated, filter.DroppedMatch)
	}
	fmt.Fprintf(w, "dropped:   %s\n", droppedSummary(r.dropped))
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
