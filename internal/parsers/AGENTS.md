# internal/parsers/AGENTS.md

The rules for the analysis side of `doubleagent`: the parser kit
(`internal/parserkit`), the parsers (one package each under
`internal/parsers/`), the registry (`internal/parserregistry`), the tests
that run over every parser (`internal/growthtests`), deduplication
(`internal/dedup`) and the rows the analysis commands write.
The rules every change keeps are in the [root `AGENTS.md`](../../AGENTS.md)
and the Go standards in [`internal/AGENTS.md`](../AGENTS.md); a parser is
written from its agent's research document, whose rules are in
[`research/AGENTS.md`](../../research/AGENTS.md).
[Parsers and the parser kit](../../docs/internals.md#parsers-and-the-parser-kit)
maps the packages.

## Analysis standards

The analysis commands run on the analyst's workstation, never on the host.

- **Attribution comes from the manifest when there is one.** Host, user,
  home and agent come from `collection.json` and `manifest.jsonl`; only a
  loose input without a manifest infers them from paths, and the output
  says so. A row attributed to a project or session takes each value from
  that record alone, never a sibling's, and omits a value it lacks. A
  record is attributed to this host or to another machine by a marker the
  format writes (a flag on the record, or its place relative to a flagged
  record), never by equality of a path, slug or name two machines can
  share.
- **Parsers select by path relative to the home.** A parser's kinds match
  `artifact.Rel`, never a hard-coded agent path, and detection lives in
  `internal/catalog` and `internal/input`. Every new input shape `detect`
  tells apart (a collection layout, an archive, a loose home or agent
  directory) gets a case in `allInputCases` in
  `internal/input/cases_test.go`, which `TestOpenReadsEachInput` runs.
- **Inputs are read, never followed.** The analysis commands follow no link
  when they read an input and create none when they extract an archive:
  `internal/input` makes only directories and regular files, each file new
  ([`docs/inputs.md`](../../docs/inputs.md)).
- **Joins are one predicate.** A one-to-one join of a secondary source
  against its primary store is one predicate: scope, text form, window and
  direction. Whatever reserves a record for one claimant (a bound, a
  competitor set, an index) is derived from that predicate per record,
  never from a second grouping that approximates it. A widened window
  extends only in the direction, and between the stores, its cause
  explains: a parser's `SecondaryTolerance` (`parserkit.ToleranceSetter`) is
  symmetric, a `Window` hook (`parserkit.HooksDeclarer`) forward only. A match
  form (the `Form` hook) maps the agent's own markup to the other store's
  form and collapses each ASCII whitespace run to one space; it never
  deletes characters the user typed, so distinct events keep distinct
  keys. A pass over every row does per-row work independent of the row
  count (a sorted index and a binary search, or one carrying pass), and
  builds a structure only some rows need only when such a row exists.
- **Parsers are validated like catalog entries.** Confirm the record shape
  against source or a real install, record it in the Parsing part of
  `research/<agent>.md`, list the fields the parser depends on in the
  package comment, and build the test records from those shapes. Extract a
  closed-source binary's protobuf descriptors with
  `tools/proto_descriptors.py` and cite their proto file in the parser's
  schema table. Decode protobuf with `internal/protowire`; open SQLite only
  through `parserkit.OpenDatabase` (`internal/sqlitefile`), which reads the
  `-wal` and `-journal` sidecars and never writes to the evidence copy.
- **Bad lines are evidence.** A bad line is never fatal; every turn before
  and after it is still emitted. Each line that does not decode becomes its
  own `system` row `parser: unparseable line (<reason>): <line>`, holding
  the line as read less its terminator, unrepaired and unshortened; a JSON
  array reader writes `unparseable record <N> (<reason>)`. A whole-file
  error (unreadable, not a JSON array, cut inside an array) is one row
  `parser: <message>`; for a file that exists but cannot be opened, a
  SQLite sidecar included, or that a line reader cannot read through, the
  row names the file and the operating system's reason in the form
  `<file> could not be read (<reason>)` (`Problems.AddUnreadable`,
  `reason.Unreadable`), never a wrong-type message, silence or a `problem:`
  line. The row carries the file's known session id and project path, an
  empty `timestamp_utc` and `source_line` set to the line, 0 when there is
  none. This covers every file a parser reads: history files, indexes and
  secondary sources.
- **Bad-line mechanics.** A reader records each problem in the parse's
  `parserkit.Problems`; the parser turns the list into rows with
  `Base.ErrorRows` where its output puts them, and never formats a bad-line
  row itself. Every filled problems list is reported, returned or passed
  on. A helper reading an optional file returns the failure beside its
  result, so only an absent file is silent and an unreadable one reaches
  the list. A problems list holds notes (a repeated member name) beside
  faults, so a parser asks `Problems.Damaged()` whether a file is damaged
  and never judges it by the list's length or its first entry.
- **Problems outside parsers.** A file an analysis command reads itself (a
  repository's config and HEAD files, an index, a settings file) that fails
  to read or parse gives a `problem:` line, never an empty result, reported
  to stdout and the JSON outputs in the same run, in the order
  [`docs/timeline-output.md`](../../docs/timeline-output.md) documents.
  `problem:` lines and `parser:` rows are output, never log records.
- **The outputs are interfaces**, as the
  [root `AGENTS.md`](../../AGENTS.md#standards-every-change-keeps) says. The
  `text` field holds the full event text, line breaks included, never
  truncated.
- **Evidence is shown as written.** In the analysis output a number from
  the evidence keeps its source text, JSON embedded in a row is compact with
  the evidence's key order, a boolean is `true` or `false`, and a null is
  empty. Text is handled by Go's rules: white space is `unicode.IsSpace`,
  length counts runes, and invalid UTF-8 becomes U+FFFD, one per bad byte.
  A value is *empty* when it is null, false, zero, an empty string or an
  empty list or object (`jsonvalue.Value.IsEmpty`), and numbers are
  compared by value (`jsonvalue.Equal`).
- **Secrets are evidence too.** A parser returns text as written whatever
  the secrets setting, and identifies only values that are secret because
  of the field they stand in (a token, key, password or cookie, through the
  kit's `SecretText`, or `SecretJSONText` for one read as JSON) and reader
  errors that can quote a file holding secrets (`ReaderErrorText`). A
  parser never tests the setting, never writes the placeholder and never
  hides a value by a wording of its own; a URL's credentials need no
  parser, since the output stage finds them in every string it writes.
  Every row is therefore the same in both modes, and grouping, joining,
  deduplicating, sorting and counting see the same values; replacement
  happens once, where output is written (`internal/secrets`). What
  `--no-secrets` replaces, and where, is in
  [Secrets in `docs/timeline-output.md`](../../docs/timeline-output.md#secrets);
  an encrypted value stays unread, and no document claims more than those
  rules withhold. The fleet CSV's leading `'` before a text that begins
  with `=`, `+`, `-`, `@`, a tab or a carriage return
  ([`docs/fleet.md`](../../docs/fleet.md)), and text printed for a person,
  which shows control characters and bytes that are not UTF-8 visibly, are
  the two exceptions to text as written.
- **Validation evidence.** The agent's subsection under
  [Parsed agents](../../docs/supported-agents.md#parsed-agents) names the real
  installs (platform, version, date) behind the parser's paths and shapes,
  and says "fixture only" for a path resting on source or a bundle alone.
  The research document holds one Observed bullet per cell
  ([Research document structure](../../research/AGENTS.md#research-document-structure)).

### What the kit's tests hold every parser to

- **A parser package builds no disk path itself.** Outside its tests it
  calls none of `filepath.Join`, `Dir`, `Clean`, `Abs`, `Rel`,
  `EvalSymlinks`, `Glob`, `Walk` and `WalkDir`, and takes every path it
  opens from the artifact, from the kit's `Beside`, `Above`, `Under` or
  `Home.PathOf`, or from the `Child` of a `parserkit.Directory`, which
  `DirectoryAt` gives after finding a plain directory. Every directory on
  the way down has then been looked at and a link refused, so a name read
  from the evidence cannot lead a parser out of the collection.
  `TestNoParserPackageBuildsAPath` in `internal/follow` fails on such a
  call; the kit is where they are made.
- **Evidence JSON is decoded through the kit.** A parser decodes the JSON
  it reads from the evidence with the kit's readers or `parserkit.DecodeJSON`,
  so an object that repeats a member name gives a `parser:` row.
  `TestParsersDecodeEvidenceThroughTheKit` fails on a call of
  `jsonvalue.Decode`, `DecodeString` or `DecodePrefix` beyond a reviewed
  list of values that hold no object; `TestParsersWalkOnlyReviewedDocuments`
  fails on a `jsonvalue.NewDocument` beyond a reviewed list, each entry
  naming where its `TakeRepeats` goes; and
  `TestParsersKeepTheRepeatsOfAStateDatabase` fails on a call of
  `vscodestate.ReadItem` or `ReadItems` that drops the repeats it returns. A
  repeat is reported once, by the parse of the file that holds it, as the
  comment of `parserkit.NoteRepeats` states;
  `TestARepeatIsReportedOnceByTheParseOfTheFileThatHoldsIt` in
  `internal/growthtests` fails on a repeat reported twice or on another file.
- **A parser's output and work grow in step with its input.** A parser lists
  every kind of file it reads (`Kinds`, from which its `Wants` is built;
  `TestWantsIsTheUnionOfTheKinds`), and a new parser or kind ships with its
  growth cases in `internal/growthtests/growth_<package>_test.go`, added to
  `growthSets()` in registry order: inputs built at two sizes in each shape
  that fits the kind (many records naming one large value, a repeated id,
  one long carried value, deep nesting and damaged siblings, many files that
  look at each other, equal join keys, directories that cannot be listed),
  with one long value for each field the parser copies into more than one
  row. `TestEveryFileKindHasGrowthCases` fails for a kind without one.
  `TestEveryParserGrowsInStepWithItsInput` reads the rows as a run writes
  them, each group of a repeated id attached to its first parent and again
  to its last and each shared text written once, and fails when at twice the
  size the rows or their bytes are more than 2.5 times those at the size, or
  the allocations or bytes allocated more than 2.5 times plus 2,048
  allocations or 256 KiB (the runtime's own work); or when at either size
  the row text (every field but host, user, agent and source file) passes
  four bytes per byte of input plus 16,896 bytes a row (four carried values
  of 4,096 bytes, and 512). Allocations are not counted under the race
  detector, and work that allocates nothing not at all; a parser's own tests
  count it (`parserkit.Work`).
- **Secrets in tests.** A parser's test of a withheld value runs the parse
  through `internal/parserkit/parsertest`, which also checks that the rows are
  the same in both secrets modes.

### Tests of a join

- A proximity join of two record kinds is bounded by the next record of the
  same kind and tested with two interleaved pairs (A, A's partner, B, B's
  partner).
- A secondary-source join is tested with each axis of its predicate and the
  inverse: the earlier and the later claimant missing, equal-time copies
  across files, a narrower and a broader scope, the same scope in another
  session, a relocated project.
- Every branch a change adds to a parser or modifies in one is taken both
  ways by its tests.

## Adding a parser

The research is done for every agent in the parser backlog; do not redo
it. Start from the Parsing part of `research/<agent>.md`, whose Parser plan
maps records to timeline columns and whose Fixtures subsection holds the
sample records. Recheck the record shape against each source the document
cites, as step 4 of
[Validating catalog entries from source](../../research/AGENTS.md#validating-catalog-entries-from-source)
says, a source file at its cited commit rather than the default branch, and
note any drift in the package comment. Rechecking a citation is not redoing
the research.

A per-step round (an install in a clean VM with a listing after every user
action; [Real-install rounds](../../lab/AGENTS.md#real-install-rounds))
comes first for an agent whose research rests on source alone, a newer agent
version, and a [surface](../../lab/AGENTS.md#real-install-rounds) not yet
exercised. Its line types and key sets go into the Record schema and Parser
plan, and the parser is written from them.

1. Create the package `internal/parsers/<name>`, named for the agent, with a
   type that embeds `parserkit.Base` (`parserkit.NewBase(agent, name)`) and
   implements `parserkit.Parser` and `parserkit.KindLister`.
   - Its agent name equals the catalog name. `Kinds` lists every kind of
     file it reads (`parserkit.PatternKind` over `artifact.Rel`), and `Wants`
     is their union. `Parse` emits rows built from `Base.BaseRow` and
     `Base.SystemRow`, which fill host, user, agent and `source_file`.
   - Its package comment says what it reads and emits, lists the fields it
     depends on, names the commit or install its shapes were validated
     against with any drift, and names the research document it follows;
     record shapes, versions and history stay in that document, and the
     comment states no fact the document lacks.
   - The optional interfaces say what else it does: `parserkit.SecondaryReader`
     for the files that repeat the primary store's events,
     `parserkit.ToleranceSetter` for a wider window, `parserkit.HooksDeclarer` for
     another match form, window or anchor, `parserkit.Marker` for the session
     marks, and `parserkit.AgentReader` for a file the catalog attributes to
     another agent; nothing else routes files across agents. A wrapper over
     another agent's format reuses that agent's reader.
   - A helper two parsers need lives in the kit or its own package, never in
     a parser package. A parser package imports another parser package only
     to reuse its reader, one way.
2. Register its constructor in `constructors` in
   `internal/parserregistry/parserregistry.go`, at its place; no existing line moves,
   since the order decides ties in the output. Nothing else discovers
   parsers.
3. Follow the shared row semantics. Before review, each new row form is
   checked against every bullet below, and a parser newly setting a mark or
   a secondary source updates every `sessions.jsonl` and inventory
   expectation (the merge step's job when the work is delegated).
   - `tool_use` rows carry `tool_name` and `tool_use_id`; `tool_result`
     rows join on `tool_use_id`; `thinking` rows only when
     `Options.IncludeThinking` is set.
   - `session_id` and `project_path` propagate from the session header to
     every row; `git_branch` stays empty when the format lacks it. A format
     without per-message timestamps inherits the session timestamp, and the
     package comment says so.
   - **Carried values.** `parserkit.Within` cuts every field of a row but its
     text that is longer than 4,096 bytes, with one `parser:` problem row per
     field. A parser passes every key, name or path it copies into the text
     of more than one row through a `parserkit.Carried`, which cuts it alike
     and reports one problem row per kind each time the parser calls
     `Report` (once a parse, or once a message or event); nothing checks this
     mechanically, so a reviewer looks for such copies. A parser never cuts a
     value itself. A `Carried.Cut` result goes into a row's text: in a field
     long enough for the kit to cut again, it is left out whole.
   - **Permission rows.** Each per-call decision (approval, denial, "allow
     always", automatic) gets a `system` row beside the call, joined on
     `tool_use_id` when possible: `permission decision: <decision> ...`.
     Each entry of a standing allow or deny list, trusted folder or sandbox
     setting in a file gets a `system` row at the file's modification time,
     saying so.
   - **Model rows.** An agent that can use or switch models gets a `system`
     row `model: <id>` at the first record of a session naming one and
     `model: <old> -> <new>` on every change, with the agent's own request
     or alias beside the resolved id when both are recorded; the `model`
     column alone is not enough. A `/model` command the user typed keeps
     its own row.
   - **Past use.** On every run, not as a fallback, the parser reads every
     file that records past use beyond the primary store: per-prompt
     histories, mirrors of the transcript (search indexes, summaries
     databases, a web interface's copy, brain or memory directories),
     archives of reset or deleted sessions, backups and text-only prompt
     histories. `Secondary` is true for those that repeat the primary
     store's events, the secondary sources, so deduplication keeps one copy
     of each event and what only the secondary source holds
     ([Secondary sources in `docs/timeline-output.md`](../../docs/timeline-output.md#secondary-sources)),
     and recovered rows and sessions are marked `recovered` in
     `timeline.jsonl`, `sessions.jsonl` and the inventory. A secondary
     source without its own record times gives rows an empty
     `timestamp_utc`, never the session's time; one that stamps events
     later than the primary store sets a wider window and says why in its
     comment.
   - **Sync, relay and origin rows.** A marker of a session uploaded to the
     vendor's account gets a `system` row
     (`session synced: remote session <id> last event <id>`, or the agent's
     own terms) at the file's modification time, saying so, and the parser
     sets `Marks.Sync`. A session carried through a messaging platform is a
     relay: it gets a `session relayed:` row and the parser sets
     `Marks.Relay`, so `sessions.jsonl` `relayed` reads `yes` or `no`. A
     session that originated on the vendor's servers and is only listed
     locally is synced and gets a `session origin: remote <service>` row;
     the parser sets `Marks.Origin`, so `origin` reads `remote` or `local`.
   - **Source-host rows.** A recorded source-host account gives a `system`
     row `source host: <host> account <login>` at the record's time. A
     cached token is a credential: the catalog flags its file, and a row
     that gives it shows it unless the run withholds secrets
     ([Secrets are evidence too](#analysis-standards)); a repository list
     is a project list.
   - **Research first.** Before the parser is written, each fixed Parser
     plan subsection of the research document names the files and fields
     behind its rows, or says the agent records none. For the model it says
     where it is recorded per message, session and switch; for past use
     what each file holds, what it survives (a deleted session, a rewind, a
     removed project, a compaction), how it joins the primary store
     (session id, project path, text and time) and whether it is primary or
     secondary; for sync, the command or setting that syncs or unsyncs; for
     a source host, where the account, its token or grant (a file, secret
     storage, the OS keychain, or server-side only), the repository list and
     repository-bound conversations live. Every row text, `parser:` message,
     file pattern and record key a parser uses appears as a code span or
     fenced line in the Parsing part of its research document; a literal a
     parser reuses from another parser's package may be named in that
     parser's document instead (`tools/check_parser_research.py` checks
     this). A commit that finds the document wrong fixes the document in
     that commit.
   - **Survival table.** Update the agent's row in the
     [survival table of `research/README.md`](../../research/README.md#where-transcripts-live-and-what-survives)
     in the same commit: one short sentence per cell with the file name,
     the one qualifier that matters, `parsed`, `secondary` or `not parsed`,
     and a link to the document's `#### What survives a delete`, which
     holds the facts; a fact the table holds that the document lacks goes
     into the document first. The row names each file the parser's
     `Secondary` accepts, as a code span or by its file name.
4. Write the package's tests over synthetic records built under
   `t.TempDir()`, with `/srv/proj` as the project path, a noise file the
   parser must not want, a cut-off line for each file read line by line
   (checking that the lines around it are emitted beside the `parser:`
   row), and both the present and the absent form of every key the parser
   branches on. A test record is derived from the Record schema and the
   samples' shapes, never invented and never a copy of a Fixtures sample,
   so the sample stays independent evidence. A binary input Go cannot build
   without the code under test goes under the package's `testdata` with the
   script that built it ([Tests](../AGENTS.md#tests)).
5. Add the growth cases of step 3's rule
   ([What the kit's tests hold every parser to](#what-the-kits-tests-hold-every-parser-to)).
6. Give the agent's files to the analysis commands' expected-output inputs
   (the fixed archives under `internal/input/testdata/fixtures` and the
   inputs under `internal/analyze/testdata/golden-inputs`), rewrite the
   expected files with `DOUBLEAGENT_UPDATE_GOLDEN=1`
   ([Expected output of the analysis commands](../../docs/testing.md#expected-output-of-the-analysis-commands)),
   rewrite the example blocks of
   [`docs/timeline-output.md`](../../docs/timeline-output.md)
   ([The output examples](../../docs/development.md#the-output-examples)),
   and read every diff.
7. In the [agents page](../../docs/supported-agents.md) table, set the agent's status to
   `parsed` and its "Validated against" cell to the evidence class and
   platforms in a few words (`real installs: macOS, Windows (October 2026)`,
   `source; fixture only`); its Notes cell keeps the research link, the open
   issue links and, only while something is unparsed, a clause of about
   twelve words. Move its subsection from
   [Collected agents](../../docs/supported-agents.md#collected-agents) to
   [Parsed agents](../../docs/supported-agents.md#parsed-agents), alphabetically, with
   "Files read:" and "Validated against:" (platform, version and date of
   every real install). Move the agent out of every example showing it under
   `detected but not parsed`, `collected only` or `parser: false`, and out
   of `NoTranscripts` if it was there. Add an `Added` line to
   [`docs/CHANGELOG.md`](../../docs/CHANGELOG.md), then run `make check`.

The backlog may be delegated one parser per subagent, in parallel. A
subagent changes no assertion outside its own package, and the merge step
re-derives the shared counts and expected files rather than taking either
branch's.
