# internal/catalog/AGENTS.md

The rules for the catalog, `internal/catalog/catalog.txt`, and the
checklist for adding an agent to it. The rules every change keeps are in
the [root `AGENTS.md`](../../AGENTS.md), the Go standards in
[`internal/AGENTS.md`](../AGENTS.md), and the research that every catalog
line is written from in [`research/AGENTS.md`](../../research/AGENTS.md).

## Catalog rules

The seven tables (`CATALOG`, `PROJECT_CATALOG`, `EXCLUDES`, `SECRET_GLOBS`,
`DOCKER_VOLUMES`, `RELOCATION_VARS`, `STARTUP_FILES`) are one embedded text
file in the format `collect --list` and `doubleagent catalog` print. A new
tool is new lines, never new code. Every catalog entry is tried against
every home directory, and every `STARTUP_FILES` line against each base of
its scope. `internal/catalog/rules_test.go` checks the mechanical rules
below; the others are the reviewer's.

- **Relocation and startup lines.** `RELOCATION_VARS` lists
  `agent|VARIABLE|replaces`: an environment variable that moves an agent's
  state, and the home-relative path its value stands in for (empty when it
  adds a store with no default under the home). A variable many programs
  honour (`XDG_CONFIG_HOME`, `ZDOTDIR`) is a `shared` line, a weaker hint. A
  variable two agents read has a line under each.
  `TestRelocationLinesNameAnUpperCaseVariableAndAHomeRelativePath` checks
  the form. `STARTUP_FILES` lists `scope|glob` for the files that set an
  environment: scope `home` (relative to each home), `system` (absolute,
  under the root in image mode) or `project` (relative to each project);
  `TestStartupFilesAreAbsoluteExactlyUnderScopeSystem` checks the form.
  `collect` searches those files for the variable names and never
  evaluates a value; a file that names one is collected as agent `shared`,
  flagged `secret: true`. [`docs/relocation.md`](../../docs/relocation.md)
  says what each kind of file and the registry give.
- **Docker volumes.** `DOCKER_VOLUMES` maps a Docker or Podman volume name
  glob to an agent (`agent-zero|*a0_usr`); of the lines matching a name,
  the one with the fewest `*` wins, then the one with the most literal
  characters, then the line that sorts first (by agent, then glob). A matched volume is collected
  whole, its `_data` directory the collection base; an unmatched one gets
  one `skipped_unmatched_volume` row. A tool in a volume adds
  exclusions and secret globs relative to the volume's root as
  `volume|glob` lines (`volume|tmp/playwright`, `volume|secrets.env`),
  which apply only below a volume's `_data`, never colliding with another
  tool's volume layout (no bare `tmp` or `cache`). A line a home or project
  entry can reach carries no prefix, since it must also apply there.
- **Glob forms.** A glob in any table holds only `*`, `?` and a closed
  bracket list of literal bytes, ranges and `!`, never a backslash, a
  caret, a character class name, a collating or equivalence class or an
  unclosed bracket: `internal/glob` matches any other form as nothing
  (`TestEmbeddedCatalogHoldsOnlyGlobFormsEveryShellReadsAlike`).
- **What a glob matches.** Catalog and project globs match one path segment
  at a time, so their `*` and `?` do not cross `/`. Exclusion and secret
  globs match the whole path relative to the collection base (a home, a
  discovered project or a volume's `_data`), so their `*` crosses `/`, and
  they may open with `*/` to match at any depth
  ([Glob matching](../../docs/internals.md#glob-matching)).
- **Anchors.** Anchor every catalog and project glob on a path component
  (`.hermes*/auth.json`, never `*hermes*/auth.json`); the one accepted
  leading `*` is the whole component `*` of `*/User/globalStorage/...`
  (`TestGlobsAreAnchoredOnAPathComponent`).
- **Secret glob characters.** A secret glob, read without its `volume|`
  prefix, may contain spaces but never `|`, `(`, `)`, `&`, `;`, `<`, `>`,
  `$`, a backtick, a quote or a backslash, and never starts with `#`
  (`TestEmbeddedSecretGlobsHoldNoCharacterTheCasePatternCannot`); the
  loader refuses any other bar in an exclusion or secret line.
- **Hygiene.** Each of the seven section headers appears once, in file
  order; every table holds at least five lines; no line ends in white space
  or holds a tab or a carriage return; and no table line repeats inside its
  table, ASCII case ignored (`rules_test.go`).
- **Exclusion beats secret.** When an exclusion and a secret glob both
  match, the exclusion wins and the directory's `skipped_excluded` is the
  subtree's only row; a credential inside needs a narrower exclusion
  (`hermes-agent/[!.]*` keeps `.env`).
- **Case.** Every glob ignores ASCII case on every platform, and a match
  is spelled as its directory lists it, so a path is listed once, in the
  spelling its research document gives for the platform the agent writes
  it on, never once per spelling or with a bracket of both cases
  (`[Zz]ed`). When an exclusion ending in a common name
  (`*/Claude*/Cache`) also prunes another tool's directory, that tool's
  entry names the directory's children
  (`AppData/Local/claude-cli-nodejs/Cache/*`), never the directory, and the
  Windows fixture image holds a file under it
  ([Case variants](../../docs/coverage.md#case-variants)).
- **Nested entries.** A nested entry is an entry inside another entry's
  match (`antigravity|.gemini/antigravity-cli` inside `gemini-cli|.gemini`).
  It claims its subtree, so `collect` and the analysis commands' reading of
  a loose tree attribute each file once, to the most specific agent. Use one
  whenever a tool stores state inside another tool's directory.
- **Order.** Every table is sorted byte by byte, field by field, never as
  whole lines (`|` sorts after `-`, so a whole-line sort puts `pi-x|`
  before `pi|`): `CATALOG`, `PROJECT_CATALOG` and `DOCKER_VOLUMES` by agent,
  then glob; `RELOCATION_VARS` by agent, variable, then replaced path;
  `EXCLUDES` and `SECRET_GLOBS` by glob, the `volume|` lines after the
  others; `STARTUP_FILES` by scope, then glob. A new line goes in its sorted place (`TestEveryTableIsSorted`).
  `catalog.Parse` sorts each table by the same keys after reading it, so
  no outcome depends on where a line sits in the file; the file is held to
  that order because `collect --list` and `catalog` print it as written,
  and code never gives meaning to the order of a table's lines beyond
  these keys.
- **Comments.** A comment run sits directly above a table line and covers
  that line and the lines of the same agent below it (of the same scope in
  `STARTUP_FILES`; only that line in `EXCLUDES` and `SECRET_GLOBS`) up to
  the next comment, and its wording is true for every line it covers: a
  fact about some of them names their globs, never their position ("the
  line above"). A comment heads one agent's lines, never several agents'
  or a vendor's. The one exception is a table's note, a run directly under
  the section header closed by a blank line, the only blank line a table
  may hold (`TestEveryCommentSitsAboveATableLine`).
  `tools/drift_common.py` attaches comments to lines by the same rule.
  `collect --list` and `catalog` print the comments, so they are written
  for the responder under the documentation rules of the
  [root `AGENTS.md`](../../AGENTS.md#documentation).

## Adding an agent to the catalog

Prefer reading the tool's source over installing it. Decide what counts as
one agent first, by the rule under
[Research document structure](../../research/AGENTS.md#research-document-structure);
the name in the `agent|path` lines, the document's file name and the
parser's agent name are the same string.

1. Shallow-clone the repository into the scratchpad, grep for the terms
   under
   [Validating catalog entries from source](../../research/AGENTS.md#validating-catalog-entries-from-source),
   read the paths module, and cite file and line in the issue. For a
   closed-source tool, follow the evidence order there (documentation, then
   the shipped bundle or binary, then forums and write-ups) and say which
   level each claim came from.
2. Identify transcripts, history, config, credentials and anything large
   (binaries, model weights, caches, indexes, shadow git checkpoints,
   worktrees, marketplace clones). Many tools use `~/.config` and
   `~/.local/share` on macOS and Windows too, and several keep credentials
   in the OS keychain with a file fallback only on headless hosts.
3. Add `agent|path` lines to `CATALOG` in `internal/catalog/catalog.txt` for
   every platform path, including the Windows `AppData` location, each in
   its sorted place under the agent's comment, and an `Added` line under
   `Unreleased` in [`CHANGELOG.md`](../../CHANGELOG.md). Add a
   `RELOCATION_VARS` line for every environment variable the tool reads to
   place its state, and list it in the research document's
   `#### Relocation variables` subsection.
4. Add large or irrelevant subtrees to `EXCLUDES` and credential files to
   `SECRET_GLOBS`.
   - A secret glob covers every copy the agent makes of a credential file
     (a dated backup, a `.corrupted` copy such as
     `.claude.json.corrupted.<ts>`, a snapshot, a rotated log). A settings
     file that can hold MCP server `env` or `headers` values is a
     credential file.
   - Exclude what the agent copied, collect what it made: worktrees,
     clones, marketplace checkouts, `node_modules`, caches, embedding
     indexes, model weights and downloaded binaries are excluded. A scratch
     or output directory (Claude Code's temp root) is collected whole,
     capped by size; a large copied subtree inside it is excluded by name.
   - A line that collects a directory whole is written against the
     directory's documented layout: list its children from the format's
     reference (`gitrepository-layout` for `.git`), mark each collected or
     excluded in the research document, and add every exclusion in the same
     commit.
   - A secret glob under an entry that names a directory's children needs
     its own catalog line (as the `.claude-*/` relocated-home globs do).
   - A named Docker volume the tool's README, compose file or installer
     creates goes in `DOCKER_VOLUMES`, its source cited in its agent's
     comment.
5. If the tool records project paths, add their extraction to the matching
   source group of project discovery
   ([Project discovery](../../docs/internals.md#project-discovery)), and any
   per-project files to `PROJECT_CATALOG`. Process names are never listed.
   A project path that lives only in a SQLite database is read by the
   parser, not by discovery.
6. Give every new line its evidence in the fixture image
   ([The fixture image](../../docs/testing.md#the-fixture-image)): add a
   file line, in sorted place, to
   `internal/collect/imagetest/testdata/image_unix.txt` and to
   `image_windows.txt`, in the home of each operating system the research
   document places the path on, never beside an entry differing from it
   only in letter case.
   - Each `CATALOG` and `PROJECT_CATALOG` line gets a fixture path it
     matches whose name its research document gives; each `EXCLUDES` line a
     directory it excludes; each `SECRET_GLOBS` line a file it flags, which
     a `--no-secrets` run skips; each `STARTUP_FILES` and `RELOCATION_VARS`
     line a startup file that names the variable; each `DOCKER_VOLUMES`
     line a volume it names.
   - The fixture holds one file per credential name in the research
     document's Credentials section, not one per glob, and a file for
     anything unusual the tool does (symlinks, SQLite sidecars, spaces,
     drive-letter or `file:///` project references). A layout the main
     image cannot hold is built by a setup method in `setups_unix.go` or
     `setups_windows.go`.
   - `TestEveryCatalogLineLeavesItsEvidence` in `internal/collect` fails
     for a line no run gives evidence of, unless
     `internal/collect/testdata/coverage-uncovered.txt` lists it with the
     reason the test derives
     ([Catalog coverage](../../docs/testing.md#catalog-coverage)). A new
     line that lands under `none` or an `-unreached` reason is a missing
     fixture, not an answer.
   - Rewrite the expected collection output on Linux, macOS and Windows
     (`DOUBLEAGENT_UPDATE_GOLDEN=1`, [Expected output of `collect`](../../docs/testing.md#expected-output-of-collect))
     and read every group of changed rows for paths the line was not meant
     to reach.
7. Write the Collection part of `research/<agent>.md`, and the Parsing part
   if the tool keeps transcripts worth parsing; add the agent to the index
   table in [`research/README.md`](../../research/README.md), with its
   Parser and Notes cells; note the tool in
   [`docs/coverage.md`](../../docs/coverage.md) if needed; give the matching
   issue a `Closes #<n>` line in the pull request body. Add the agent to the
   [agents page](../../docs/supported-agents.md) table as `backlog` (parseable
   records, with a parser issue) or `collected` (nothing to parse, with the
   reason), linked to a new subsection under
   [Collected agents](../../docs/supported-agents.md#collected-agents) listing what is
   collected. A `collected` agent also goes into `NoTranscripts` in
   `internal/parsers/parsers.go`, which the analysis summary reports as
   `collected only`. A parser issue filed later is linked in the row by the
   commit that files it, and examples of the agent list (`agents:`,
   `detected_not_parsed`, `collected_only`) change when the agent appears in
   a fixture.
8. Run `tools/check_tables_research.py`, `make check` and a live collection
   ([Validation and testing](../../AGENTS.md#validation-and-testing)),
   reading the `skipped_excluded` and `secret: true` rows.

Source validation gets the lines into the tables; a per-step real-install
round on every platform and surface the agent ships, one listing diff (the
change between listings of the agent's directories before and after a user
action) per user action, completes the addition
([Real-install rounds](../../lab/AGENTS.md#real-install-rounds)). An agent
added from source alone gets its round filed as an issue, and its evidence
cell in the index table names a real install only after the round.
