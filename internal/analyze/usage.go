package analyze

import "github.com/seanthegeek/doubleagent/internal/rowfilter"

// inputArgumentHelp is the help of the input detect and timeline share.
const inputArgumentHelp = `
  INPUT                an archive doubleagent collect wrote, or any tar
                       (plain, gzip or bzip2) or zip archive; an extracted
                       collection; or any directory
`

// inputSharedOptionsHelp is the help of the options detect and timeline
// share before --no-secrets.
const inputSharedOptionsHelp = `  --host HOST          host name to record for every home, in place of
                       the hostname collection.json names
  --user USER          user name to record for a home whose owner cannot be
                       inferred
  --work-dir DIR       where to extract an archive (default: a temporary
                       directory removed afterwards)
  --keep-extracted     do not delete the extracted archive
`

// timelineNoSecretsHelp is the help of timeline's --no-secrets.
const timelineNoSecretsHelp = `  --no-secrets         replace each identified secret with REDACTED: the
                       credentials of any URL that has a scheme, wherever
                       the output writes it, and the value of a URL
                       parameter named as a credential (token, key, sig
                       and the like); and the tokens, keys and reader
                       errors the parsers identify. A token, key or URL
                       credential of 8 bytes or more is replaced
                       everywhere it appears in the output; a shorter URL
                       credential only where the URL holds it, and a
                       shorter token or key only in the text of the rows
                       of the parse that read it. A reader's error is
                       replaced only in the text of its parse's rows and
                       in its problem lines
`

// detectNoSecretsHelp is the help of detect's --no-secrets: detect reads
// none of the agents' own files, so no parser identifies a token or key.
const detectNoSecretsHelp = `  --no-secrets         replace each credential of a URL that has a scheme
                       with REDACTED, and the value of a URL parameter
                       named as a credential (token, key, sig and the
                       like), wherever the output writes the URL: a
                       credential of 8 bytes or more is replaced
                       everywhere it appears in the output, a shorter one
                       only where the URL holds it. detect reads none of
                       the agents' own files, only a collection's
                       manifest.jsonl and collection.json and each
                       project's .git/config and .git/HEAD, so a token or
                       key that timeline --no-secrets identifies by the
                       parsers is not replaced here, even in a file name
                       or a path
`

// inputPlaceholderHelp is the help of the options detect and timeline
// share after --no-secrets.
const inputPlaceholderHelp = `  --redaction-placeholder TEXT
                       with --no-secrets, replace each secret with TEXT
                       rather than REDACTED; "" leaves nothing in its place
  -h, --help           print this text and exit
`

// timelineOptionsHelp is the help of the options timeline shares with
// detect, with timeline's own --no-secrets.
const timelineOptionsHelp = inputSharedOptionsHelp + timelineNoSecretsHelp + inputPlaceholderHelp

// inputLimitHelp is the note detect and timeline give about a relative
// input: it is read at its absolute path, which the system limits.
const inputLimitHelp = `
On Linux and macOS an input is read at its absolute path, so a relative
INPUT fails with "file name too long" when the working directory plus INPUT
is longer than the system's path length; give it from a shorter directory.
`

// secretsHelp is the warning detect and timeline give about the secrets
// their output holds.
const secretsHelp = `
By default the output shows secrets as the evidence holds them, so
timeline.jsonl, sessions.jsonl, inventory.json and the output of detect
hold live credentials. --no-secrets cannot guarantee that every credential
is removed: one written in free text that no rule identifies (a prompt, a
command line, a tool's output, a pasted file) is shown either way. Review a
timeline before it is shared, whichever mode wrote it.
`

// detectInputHelp is the help of the input and the options detect shares
// with timeline, with detect's own --no-secrets.
const detectInputHelp = inputArgumentHelp + inputSharedOptionsHelp + detectNoSecretsHelp + inputPlaceholderHelp

// detectUsage is the help text of detect.
const detectUsage = `Usage: doubleagent detect [options] INPUT

List the homes, users and agents found in the input. Exit status: 0 when
agent files were found or --json was given, 1 when none were, 2 on an
error, 130 when interrupted.
` + detectInputHelp + `  --json               machine-readable output
  --files              list every attributed file
` + inputLimitHelp + secretsHelp

// timelineUsage is the help text of timeline.
const timelineUsage = `Usage: doubleagent timeline [options] INPUT OUTPUT_DIR

Parse transcripts into timeline.jsonl, sessions.jsonl and inventory.json.
Exit status: 0 when rows were written, 1 when there were none, 2 on an
error, 130 when interrupted.
` + inputArgumentHelp + `  OUTPUT_DIR           a new or empty directory for the output files; its
                       path is cleaned by name before it is judged and
                       written, so link/../out names out
` + timelineOptionsHelp + `  --include-thinking   emit thinking and reasoning blocks as rows of type
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
` + inputLimitHelp + secretsHelp

// fleetUsage is the help text of fleet.
const fleetUsage = `Usage: doubleagent fleet [-o CSV] INPUT [INPUT ...]

Merge saved outputs of collect --inventory, or directories of them, into one
CSV with a row per host and agent, and print a rollup per agent. Exit
status: 0 when rows were written, 1 when there were none, 2 on an error,
130 when interrupted.

  INPUT                a saved stdout of collect --inventory, or a directory
                       of them
  -o, --output CSV     CSV to write (default: ` + defaultFleetName + ` in the
                       current directory)
  -h, --help           print this text and exit

The CSV is refused when it is a symbolic link, an input file, or inside an
input directory, judged where the system puts it when the file is written:
on Linux and macOS a .. after a link leads up from the link's target, and
on Windows a .. cancels the name written before it. It is refused too when
where it lies cannot be told, such as through a junction or link on Windows.
The CSV is written to a new file beside it, named doubleagent-fleet-*.partial,
then moved into its place, so an existing CSV is replaced whole, never
written into, and its directory must be writable. A .partial file left
behind by a run killed while writing can be deleted. When the CSV cannot be
written, the directories made for it are removed again while empty, unless the
directory part of the name holds a . or .. element; then they are left.

A CSV cell that begins with =, +, -, @, a tab or a carriage return is
written with a ' in front, so a spreadsheet shows it as text; the saved
inventories and the rollup printed here keep it as written.
`

// catalogUsage is the help text of catalog.
const catalogUsage = `Usage: doubleagent catalog [--agents]

Print the bundled artifact catalog.

  --agents    print the agent names and whether each has a parser
  -h, --help  print this text and exit
`

// usageOf returns the help text of the analysis command called name, or
// catalog's for a name that is not one of Commands.
func usageOf(name string) string {
	if found, ok := commandNamed(name); ok {
		return found.usage
	}
	return catalogUsage
}
