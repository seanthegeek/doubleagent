//go:build parity

package replay

import (
	"slices"
	"testing"
)

// rewriteOf returns the text rewrite of difference number for one side. A
// difference whose entries belong to one parser each rewrites the text as a
// row of the agent of the first entry either of whose forms shows in it,
// and as a command's output when none does.
func rewriteOf(t *testing.T, number int, from side) func(text string) (string, int) {
	t.Helper()
	entry := difference(t, number)
	plain := entry.Text
	if from == binarySide {
		plain = entry.Binary
	}
	if entry.RowText == nil {
		return plain
	}
	return func(text string) (string, int) {
		for _, form := range slices.Concat(withheldForms(), shownForms()) {
			_, recorded := form.rowRewrite(recordedSide, form.agent, text, caseRun{})
			_, binary := form.rowRewrite(binarySide, form.agent, text, caseRun{})
			if recorded+binary > 0 {
				return entry.rowRewrite(from)(form.agent, text)
			}
		}
		return plain(text)
	}
}

// textDifferenceCase is one case of a difference that rewrites text: the text
// of one side rewritten as the difference's number rewrites it, the rewrite
// wanted, and the places found.
type textDifferenceCase struct {
	name   string
	number int
	side   side
	text   string
	want   string
	found  int
}

// jsonReaderCases returns the cases of difference 102 for the readers of JSON: the
// decoders' details, a thread that is not an object, a worktree record without
// its seconds, and the analysis commands' own wording.
func jsonReaderCases() []textDifferenceCase {
	return []textDifferenceCase{
		{"102 rewrites a recorded JSON decoder detail keeping its position", 102, recordedSide,
			"parser: unparseable line (Expecting ',' delimiter: line 1 column 5 (char 4)): {\"a\" 1",
			"parser: unparseable line ({json error}): {\"a\" 1", 1},
		{"102 rewrites the binary's JSON decoder detail keeping its position", 102, binarySide,
			"parser: unparseable record 2 (invalid JSON at line 1, column 4: invalid character ']' looking for " +
				"beginning of value): ]",
			"parser: unparseable record 2 ({json error}): ]", 1},
		{"102 rewrites the binary's end of JSON input", 102, binarySide,
			"parser: unparseable record 2 (invalid JSON at line 1, column 9: unexpected end of JSON input): [1,",
			"parser: unparseable record 2 ({json error}): [1,", 1},
		{"102 leaves a lead-in in capitals before the recorded JSON detail", 102, recordedSide,
			"parser: unparseable record 4 (tool_calls is not valid JSON: Expecting value: line 1 column 1 (char 0)): nope",
			"parser: unparseable record 4 (tool_calls is not valid JSON: {json error}): nope", 1},
		{"102 leaves a record's key before the recorded JSON detail", 102, recordedSide,
			"parser: unparseable record 14 (bubbleId:Ab: Expecting value: line 1 column 1 (char 0)): x",
			"parser: unparseable record 14 (bubbleId:Ab: {json error}): x", 1},
		{"102 leaves a note after the binary's JSON detail", 102, binarySide,
			"parser: unparseable record 2 (invalid JSON at line 1, column 1: invalid character '(' looking for " +
				"beginning of value; stored zstd blob as hex): 28b5",
			"parser: unparseable record 2 ({json error}; stored zstd blob as hex): 28b5", 1},
		{"102 rewrites Python's JSON error with its class name", 102, recordedSide,
			"parser: unparseable record 2 (JSONDecodeError: Expecting value: line 1 column 1 (char 0); stored zstd " +
				"blob as hex): 28b5",
			"parser: unparseable record 2 ({json error}; stored zstd blob as hex): 28b5", 1},
		{"102 rewrites Python's thread that is not an object", 102, recordedSide,
			`parser: unparseable record 2 (ValueError: not a JSON object): ["not"]`,
			`parser: unparseable record 2 ({thread not an object}): ["not"]`, 1},
		{"102 rewrites the binary's thread that is not an object", 102, binarySide,
			`parser: unparseable record 2 (the thread is not a JSON object): ["not"]`,
			`parser: unparseable record 2 ({thread not an object}): ["not"]`, 1},
		{"102 rewrites Python's worktree record without its seconds", 102, recordedSide,
			`parser: unparseable record 6 (created_git_worktrees: KeyError: 'created_at_seconds'): {}`,
			`parser: unparseable record 6 (created_git_worktrees: {no creation time}): {}`, 1},
		{"102 rewrites the binary's worktree record without its seconds", 102, binarySide,
			`parser: unparseable record 6 (created_git_worktrees: no created_at_seconds field): {}`,
			`parser: unparseable record 6 (created_git_worktrees: {no creation time}): {}`, 1},
		{"102 leaves the analysis commands' own wording alone", 102, recordedSide,
			"parser: unparseable line (not a JSON object): [1]", "parser: unparseable line (not a JSON object): [1]", 0},
	}
}

// textReaderCases returns the cases of difference 102 for the readers of UTF-8
// text and of command lines, and for the path of a mutation not applied.
func textReaderCases() []textDifferenceCase {
	return []textDifferenceCase{
		{"102 rewrites Python's UTF-8 codec error", 102, recordedSide,
			"parser: unparseable line ('utf-8' codec can't decode byte 0xc3 in position 0: invalid continuation byte): x",
			"parser: unparseable line ({utf-8 error}): x", 1},
		{"102 rewrites the binary's UTF-8 error", 102, binarySide,
			"parser: unparseable line (not valid UTF-8 at byte offset 10): x",
			"parser: unparseable line ({utf-8 error}): x", 1},
		{"102 leaves a UTF-8 error that names no offset alone", 102, binarySide,
			"parser: unparseable line (not valid UTF-8): x", "parser: unparseable line (not valid UTF-8): x", 0},
		{"102 writes Python's path of a mutation not applied as its elements", 102, recordedSide,
			`parser: mutation not applied (path ['nowhere', 0] not found): {"kind": 1}`,
			`parser: mutation not applied (path {mutation path <nowhere> 0} not found): {"kind": 1}`, 1},
		{"102 writes the binary's path of a mutation not applied as its elements", 102, binarySide,
			`parser: mutation not applied (path ["nowhere",0] not found): {"kind": 1}`,
			`parser: mutation not applied (path {mutation path <nowhere> 0} not found): {"kind": 1}`, 1},
		{"102 reads the binary's path inside a line of JSON", 102, binarySide,
			`{"text":"parser: mutation not applied (path [\"nowhere\"] not found)"}`,
			`{"text":"parser: mutation not applied (path {mutation path <nowhere>} not found)"}`, 1},
		{"102 rewrites Python's unclosed quote", 102, recordedSide,
			"parser: unparseable line (No closing quotation): permissions allow 'x",
			"parser: unparseable line ({unclosed quote}): permissions allow 'x", 1},
		{"102 rewrites the binary's unclosed quote", 102, binarySide,
			"parser: unparseable line (a quote is not closed): permissions allow 'x",
			"parser: unparseable line ({unclosed quote}): permissions allow 'x", 1},
	}
}

// databaseAndSystemErrorCases returns the cases of difference 102 for SQLite
// databases and for a system error, each up to the end of its detail.
func databaseAndSystemErrorCases() []textDifferenceCase {
	return []textDifferenceCase{
		{"102 rewrites a SQLite error up to a JSON quote", 102, recordedSide,
			`{"text":"parser: SQLite error: file is not a database","source_line":0}`,
			`{"text":"parser: SQLite error: {sqlite error}","source_line":0}`, 1},
		{"102 rewrites the binary's SQLite error the same way", 102, binarySide,
			"parser: SQLite error: malformed database: bad header", "parser: SQLite error: {sqlite error}", 1},
		{"102 keeps the parenthesis a SQLite error ends", 102, binarySide,
			"parser: tasks.db unreadable (SQLite error: no such table)", "parser: tasks.db unreadable ({sqlite error})", 2},
		{"102 writes a recorded system error in the binary's lower case", 102, recordedSide,
			"parser: settings.json could not be read (Permission denied)",
			"parser: settings.json could not be read (permission denied)", 1},
		{"102 leaves the binary's system error as written", 102, binarySide,
			"parser: settings.json could not be read (permission denied)",
			"parser: settings.json could not be read (permission denied)", 0},
		{"102 rewrites the database detail of an unreadable OpenCode store", 102, recordedSide,
			"parser: database could not be read: file is not a database",
			"parser: database could not be read: {sqlite error}", 1},
		{"102 rewrites the database detail of an unreadable editor state store", 102, eitherSide,
			"parser: state.vscdb unreadable: database disk image is malformed",
			"parser: state.vscdb unreadable: {sqlite error}", 1},
		{"the note for an unreadable persisted output keeps its form", 102, recordedSide,
			"output\n[persisted output could not be read (Permission denied)]",
			"output\n[persisted output could not be read (permission denied)]", 1},
		{"102 leaves an assistant row that quotes an error alone", 102, recordedSide,
			"SQLite error: file is not a database", "SQLite error: file is not a database", 0},
	}
}

// archiveAndMarkupReaderCases returns the cases of difference 102 for the readers of
// TOML, zip, base64 and XML.
func archiveAndMarkupReaderCases() []textDifferenceCase {
	return []textDifferenceCase{
		{"102 rewrites the binary's TOML reader detail in a parser row", 102, binarySide,
			"parser: unreadable automation.toml: invalid TOML at line 2, column 20: strings cannot contain newlines",
			"parser: unreadable automation.toml: {toml error}", 1},
		{"102 rewrites the recorded TOML reader detail", 102, recordedSide,
			"parser: unreadable automation.toml: Illegal character '\\n' (at line 2, column 25)",
			"parser: unreadable automation.toml: {toml error}", 1},
		{"102 rewrites a recorded zip reader detail", 102, recordedSide,
			"parser: report.zip is not a zip archive (File is not a zip file)",
			"parser: report.zip is not a zip archive ({not a zip})", 1},
		{"102 rewrites the binary's zip reader detail", 102, binarySide,
			"parser: report.zip is not a zip archive (not a valid zip archive)",
			"parser: report.zip is not a zip archive ({not a zip})", 1},
		{"102 pairs a recorded member offset before the archive's start", 102, recordedSide,
			"parser: r.zip member report.json could not be read (negative seek value -4)",
			"parser: r.zip member report.json could not be read ({offset before the start})", 1},
		{"102 pairs the binary's member header placed before the archive's start", 102, binarySide,
			"parser: r.zip member report.json could not be read (header placed before the start of the archive)",
			"parser: r.zip member report.json could not be read ({offset before the start})", 1},
		{"102 rewrites a recorded base64 decoder detail", 102, recordedSide,
			"parser: session data is not base64 (Only base64 data is allowed)",
			"parser: session data is not base64 ({base64 error})", 1},
		{"102 rewrites the binary's base64 decoder detail", 102, binarySide,
			"parser: session data is not base64 (invalid base64 data at byte 4)",
			"parser: session data is not base64 ({base64 error})", 1},
		{"102 rewrites a recorded XML parse error", 102, recordedSide,
			"parser: cody_history.xml did not parse; read 0 chat(s) (not well-formed (invalid token): line 1, column 5)",
			"parser: cody_history.xml did not parse; read 0 chat(s) ({xml error})", 1},
		{"102 rewrites the binary's XML parse error", 102, binarySide,
			"parser: cody_history.xml did not parse; read 0 chat(s) (expected element name after <: line 1, column 2)",
			"parser: cody_history.xml did not parse; read 0 chat(s) ({xml error})", 1},
	}
}

// quotedValueCases returns the cases of difference 120: a value quoted in a
// message of either side.
func quotedValueCases() []textDifferenceCase {
	return []textDifferenceCase{
		{"120 writes Python's quoted value in one form", 120, recordedSide,
			"error: cannot read 'last friday' as a time; accepted: 'ago'\n",
			"error: cannot read {quoted last friday} as a time; accepted: 'ago'\n", 1},
		{"120 writes Go's quoted value in the same form", 120, binarySide,
			"error: cannot read \"last friday\" as a time; accepted: 'ago'\n",
			"error: cannot read {quoted last friday} as a time; accepted: 'ago'\n", 1},
		{"120 reads a value that starts the reason", 120, recordedSide, "error: '2026-13' is out of range\n",
			"error: {quoted 2026-13} is out of range\n", 1},
		{"120 leaves a value holding a quote alone", 120, binarySide, "error: cannot read \"it's\" as a time\n",
			"error: cannot read \"it's\" as a time\n", 0},
		{"120 leaves a quote inside a row's text alone", 120, recordedSide, "parser: cannot read 'x' here",
			"parser: cannot read 'x' here", 0},
	}
}

// withheldReasonCases returns the cases of difference 230: a reader's reason
// that the binary withholds, written with its position.
func withheldReasonCases() []textDifferenceCase {
	return []textDifferenceCase{
		{"230 gives a TOML config's recorded reason its position", 230, recordedSide,
			"parser: config.toml is not valid TOML (Illegal character '\\n' (at line 2, column 25))",
			"parser: config.toml is not valid TOML ({withheld at line 2, column 25})", 1},
		{"230 gives the binary's withheld TOML reason the same form", 230, binarySide,
			"parser: config.toml is not valid TOML (invalid TOML at line 2, column 25: " + replayPlaceholder + ")",
			"parser: config.toml is not valid TOML ({withheld at line 2, column 25})", 1},
		{"230 leaves a TOML reason that shows the reader's detail alone", 230, binarySide,
			"parser: config.toml is not valid TOML (invalid TOML at line 2, column 25: expected value)",
			"parser: config.toml is not valid TOML (invalid TOML at line 2, column 25: expected value)", 0},
		{"230 gives a JSON config's recorded reason its position", 230, recordedSide,
			"parser: config.json did not parse: Expecting value: line 1 column 11 (char 10)",
			"parser: config.json did not parse: {withheld at line 1, column 11}", 1},
		{"230 gives the binary's withheld JSON config reason the same form", 230, binarySide,
			"parser: config.json did not parse: invalid JSON at line 1, column 11: " + replayPlaceholder,
			"parser: config.json did not parse: {withheld at line 1, column 11}", 1},
		{"230 stops at the end of the text in a line of JSON", 230, binarySide,
			`{"text":"parser: config.json did not parse: invalid JSON at line 1, column 11: ` + replayPlaceholder + `","user":"alice"}`,
			`{"text":"parser: config.json did not parse: {withheld at line 1, column 11}","user":"alice"}`, 1},
		{"230 marks the binary's withheld reason of a crush.json", 230, binarySide,
			"parser: .config/crush/crush.json is not a JSON object (invalid JSON at line 1, column 3: " + replayPlaceholder + ")",
			"parser: .config/crush/crush.json is not a JSON object{added crush.json reader error}", 1},
		{"230 keeps a crush.json reason that shows the reader's detail", 230, binarySide,
			"parser: .config/crush/crush.json is not a JSON object (invalid JSON at line 1, column 3: x)",
			"parser: .config/crush/crush.json is not a JSON object (invalid JSON at line 1, column 3: x)", 0},
		{"230 marks a recorded crush.json row", 230, recordedSide,
			"parser: .config/crush/crush.json is not a JSON object",
			"parser: .config/crush/crush.json is not a JSON object{added crush.json reader error}", 1},
		{"230 gives an exec config's recorded reason and note its position", 230, recordedSide,
			"parser: exec_approvals_config raw_json unparseable (Expecting ',' delimiter: line 1 column 60 (char 59); " +
				"text withheld, it holds the approvals socket token)",
			"parser: exec_approvals_config raw_json unparseable ({withheld at line 1, column 60})", 1},
		{"230 gives the binary's withheld exec config reason the same form", 230, binarySide,
			"parser: exec_approvals_config raw_json unparseable (invalid JSON at line 1, column 60: " + replayPlaceholder + ")",
			"parser: exec_approvals_config raw_json unparseable ({withheld at line 1, column 60})", 1},
		{"230 leaves another file's reason alone", 230, binarySide,
			"parser: projects.json did not parse: invalid JSON at line 1, column 2: " + replayPlaceholder,
			"parser: projects.json did not parse: invalid JSON at line 1, column 2: " + replayPlaceholder, 0},
	}
}

// epochUnitAndPluralCases returns the cases of difference 140, the wording of the
// epoch units, and of difference 122, the plural of a manifest note.
func epochUnitAndPluralCases() []textDifferenceCase {
	return []textDifferenceCase{
		{"140 gives the recorded epoch units one form", 140, recordedSide,
			"error: cannot read {quoted soon} as a time; accepted: ISO 8601 such as 2026-10-01; 2026-10 or 2026; " +
				"epoch seconds; today\n",
			"error: cannot read {quoted soon} as a time; accepted: ISO 8601 such as 2026-10-01; 2026-10 or 2026; " +
				"{epoch units}; today\n", 1},
		{"140 gives the binary's epoch units the same form", 140, binarySide,
			"error: x; accepted: ISO 8601; 2026-10 or 2026; epoch seconds or milliseconds; today\n",
			"error: x; accepted: ISO 8601; 2026-10 or 2026; {epoch units}; today\n", 1},
		{"140 leaves epoch seconds elsewhere alone", 140, recordedSide, "parser: epoch seconds; today",
			"parser: epoch seconds; today", 0},
		{"122 writes the recorded plural note of one manifest row in the singular", 122, recordedSide,
			"note:      1 manifest rows did not parse\n", "note:      1 manifest row did not parse\n", 1},
		{"122 finds the recorded singular note too", 122, recordedSide, `"notes": ["1 manifest row did not parse"]`,
			`"notes": ["1 manifest row did not parse"]`, 1},
		{"122 leaves a note of eleven rows alone", 122, recordedSide, "note: 11 manifest rows did not parse",
			"note: 11 manifest rows did not parse", 0},
	}
}

func TestTextDifferencesRewriteOnlyTheirShape(t *testing.T) {
	tests := slices.Concat(jsonReaderCases(), textReaderCases(), databaseAndSystemErrorCases(),
		archiveAndMarkupReaderCases(), quotedValueCases(), withheldReasonCases(), epochUnitAndPluralCases())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sides := []side{test.side}
			if test.side == eitherSide {
				sides = []side{recordedSide, binarySide}
			}
			for _, from := range sides {
				got, found := rewriteOf(t, test.number, from)(test.text)
				if got != test.want || found != test.found {
					t.Errorf("side %d: got %q (%d places), want %q (%d)", from, got, found, test.want, test.found)
				}
			}
		})
	}
}

// The binary printing the recorded wording of a condition, where its own
// wording differs, still differs once each side is rewritten.
func TestTheBinarysSideRefusesTheRecordedWording(t *testing.T) {
	tests := []struct {
		name   string
		number int
		text   string
	}{
		{"a recorded JSON decoder detail", 102, "parser: unparseable line (Expecting value: line 1 column 1 (char 0)): x"},
		{"a recorded system error in capitals", 102, "parser: settings.json could not be read (Permission denied)"},
		{"a recorded XML reader detail", 102,
			"parser: cody_history.xml did not parse; read 0 chat(s) (syntax error: line 1, column 0)"},
		{"a recorded TOML reader detail", 102, "parser: unreadable a.toml: Invalid value (at line 2, column 7)"},
		{"a recorded zip reader detail", 102, "parser: r.zip is not a zip archive (File is not a zip file)"},
		{"a recorded base64 detail", 102, "parser: session data is not base64 (Incorrect padding)"},
		{"a recorded zstd detail", 102, "parser: zstd: file ends inside a zstd frame"},
		{"a recorded thread that is not an object", 102, "parser: unparseable record 2 (ValueError: not a JSON object): []"},
		{"a recorded worktree without its seconds", 102,
			"parser: unparseable record 6 (created_git_worktrees: KeyError: 'created_at_seconds'): {}"},
		{"a recorded UTF-8 codec error", 102,
			"parser: unparseable line ('utf-8' codec can't decode byte 0xc3 in position 0: invalid start byte): x"},
		{"a recorded mutation path", 102, "parser: mutation not applied (path ['nowhere'] not found): {}"},
		{"a recorded unclosed quote", 102, "parser: unparseable line (No closing quotation): 'x"},
		{"a recorded single-quoted value", 120, "error: cannot read 'soon' as a time\n"},
		{"a recorded note of one manifest rows", 122, "note:      1 manifest rows did not parse\n"},
		{"a recorded JSON config reason", 230, "parser: config.json did not parse: Expecting value: line 1 column 1 (char 0)"},
		{"a recorded TOML config reason", 230,
			"parser: config.toml is not valid TOML (Invalid value (at line 2, column 7))"},
		{"a recorded exec config reason", 230,
			"parser: exec_approvals_config raw_json unparseable (Extra data: line 1 column 3 (char 2); text withheld, " +
				"it holds the approvals socket token)"},
		{"a recorded remote with its credentials", 230,
			"  remotes: origin https://alice:FIXTURE-TOKEN@github.com/alice/proj.git\n"},
		{"recorded epoch units", 140, "error: x; accepted: ISO 8601; 2026-10 or 2026; epoch seconds; today\n"},
		{"a recorded missing record at the end", 160,
			"parser: unparseable record 3 (Expecting value: line 1 column 9 (char 8))"},
		{"recorded data after the array", 160, "parser: unparseable data after the JSON array"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want, _ := rewriteOf(t, test.number, recordedSide)(test.text)
			got, _ := rewriteOf(t, test.number, binarySide)(test.text)
			if want == got {
				t.Errorf("the binary printing %q passes as %q", test.text, got)
			}
		})
	}
}

func TestQuotedValuesStillDifferWhenTheValueDiffers(t *testing.T) {
	want, _ := quotedValueText(singleQuoted)("error: cannot read 'last friday' as a time\n")
	got, _ := quotedValueText(doubleQuoted)("error: cannot read \"last monday\" as a time\n")
	if want == got {
		t.Errorf("two different values compare equal: %q", want)
	}
}

// Each shape of difference 102 takes its detail and nothing beside it: a
// row that differs outside the detail, in a file name, a key, a count or
// a clause after it, still differs once both sides are rewritten.
func TestLibraryShapesTakeOnlyTheirDetail(t *testing.T) {
	tests := []struct {
		name      string
		want, got string
	}{
		{"an XML error of another file", "parser: history.xml: line 1, column 5",
			"parser: other.xml: line 9, column 9"},
		{"an XML error with another chat count", "parser: cody_history.xml did not parse; read 1 chat(s) " +
			"(syntax error: line 1, column 5)", "parser: cody_history.xml did not parse; read 2 chat(s) " +
			"(unexpected EOF: line 1, column 5)"},
		{"a JSON error with another key before it", "parser: unparseable record 1 (a: Expecting value: line 1 " +
			"column 1 (char 0)): x", "parser: unparseable record 1 (b: invalid JSON at line 1, column 1: invalid " +
			"character 'x' looking for beginning of value): x"},
		{"a JSON error with a clause after it", "parser: unparseable line (Expecting value: line 1 column 1 " +
			"(char 0)): x", "parser: unparseable line (invalid JSON at line 1, column 1: invalid character 'x' " +
			"looking for beginning of value, and more): x"},
		{"a TOML error of another file", "parser: unreadable a.toml: Invalid value (at line 2, column 7)",
			"parser: unreadable b.toml: invalid TOML at line 2, column 7: x"},
		{"a TOML message that is not the reader's", "parser: unreadable Config.toml: x (at line 2, column 7)",
			"parser: unreadable Other.toml: x (at line 2, column 7)"},
		{"a zstd error followed by raw text", "parser: unparseable record 2 (zstd: zstd decompressor error: x; " +
			"stored zstd blob as hex): 28b5", "parser: unparseable record 2 (zstd: corrupt data (y); stored zstd " +
			"blob as hex): 28b6"},
		{"a cut-off zstd frame for corrupt data", "parser: zstd: file ends inside a zstd frame",
			"parser: zstd: corrupt data (bad block)"},
		{"a zip error of another kind", "parser: r.zip is not a zip archive (File is not a zip file)",
			"parser: r.zip is not a zip archive (checksum mismatch)"},
		{"a mutation path of other elements", "parser: mutation not applied (path ['nowhere'] not found): {}",
			"parser: mutation not applied (path [\"elsewhere\"] not found): {}"},
		{"a system error of another condition", "parser: a.json could not be read (Permission denied)",
			"parser: a.json could not be read (no such file or directory)"},
		{"a system error of another file", "parser: a.json could not be read (Permission denied)",
			"parser: b.json could not be read (permission denied)"},
		{"a SQLite error with a clause after its parenthesis", "parser: x unreadable (SQLite error: a)",
			"parser: x unreadable (SQLite error: a), and more"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want, _ := libraryText(recordedSide)(test.want)
			got, _ := libraryText(binarySide)(test.got)
			if want == got {
				t.Errorf("%q and %q both became %q", test.want, test.got, want)
			}
		})
	}
}
