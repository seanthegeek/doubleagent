package analyze

import "github.com/seanthegeek/doubleagent/internal/rowfilter"

// inputArgumentHelp is the help of the input detect and timeline share.
const inputArgumentHelp = `
  INPUT                collector archive (.tar.gz or .zip), extracted
                       collection, or any directory
`

// inputOptionsHelp is the help of the options detect and timeline share.
const inputOptionsHelp = `  --host HOST          host name to record when the input has no
                       collection.json
  --user USER          user name to record for a home whose owner cannot be
                       inferred
  --work-dir DIR       where to extract an archive (default: a temporary
                       directory removed afterwards)
  --keep-extracted     do not delete the extracted archive
  -h, --help           print this text and exit
`

// inputHelp is the help of the input and options detect and timeline share.
const inputHelp = inputArgumentHelp + inputOptionsHelp

// detectUsage is the help text of detect.
const detectUsage = `Usage: doubleagent detect [options] INPUT

List the homes, users and agents found in the input. Exit status: 0 when
agent files were found or --json was given, 1 when none were, 2 on an
error, 130 when interrupted.
` + inputHelp + `  --json               machine-readable output
  --files              list every attributed file
`

// timelineUsage is the help text of timeline.
const timelineUsage = `Usage: doubleagent timeline [options] INPUT OUTPUT_DIR

Parse transcripts into timeline.jsonl, sessions.jsonl and inventory.json.
Exit status: 0 when rows were written, 1 when there were none, 2 on an
error, 130 when interrupted.
` + inputArgumentHelp + `  OUTPUT_DIR           a new or empty directory for the output files
` + inputOptionsHelp + `  --include-thinking   emit thinking and reasoning blocks as rows of type
                       thinking
  --agent AGENT        only parse this agent (repeatable); default is every
                       agent with a parser
  --since WHEN         keep rows at or after WHEN
  --until WHEN         keep rows at or before WHEN. UTC unless an offset is
                       given; a value without a time of day (2026-10-01,
                       2026-10, today) covers that whole period
  --keep-undated       with --since or --until, also keep rows that have no
                       timestamp
  --match REGEX        keep rows whose text matches this Go regular
                       expression (repeatable, any may match); a matched
                       tool call keeps its result and a matched result its
                       call
  -i, --ignore-case    make --match case-insensitive

WHEN is one of the forms ` + rowfilter.FormsHelp + `.
`

// fleetUsage is the help text of fleet.
const fleetUsage = `Usage: doubleagent fleet [-o CSV] INPUT [INPUT ...]

Merge saved outputs of collect --inventory, or directories of them, into one
CSV with a row per host and agent, and print a rollup per agent. Exit
status: 0 when rows were written, 1 when there were none, 2 on an error,
130 when interrupted.

  INPUT                a saved stdout of collect --inventory, or a directory
                       of them
  -o, --output CSV     CSV to write (default: ` + DefaultFleetName + ` in the
                       current directory)
  -h, --help           print this text and exit
`

// catalogUsage is the help text of catalog.
const catalogUsage = `Usage: doubleagent catalog [--agents]

Print the bundled artifact catalog.

  --agents    print the agent names and whether each has a parser
  -h, --help  print this text and exit
`

// usageOf returns the help text of a command.
func usageOf(command string) string {
	switch command {
	case "detect":
		return detectUsage
	case "timeline":
		return timelineUsage
	case "fleet":
		return fleetUsage
	}
	return catalogUsage
}
