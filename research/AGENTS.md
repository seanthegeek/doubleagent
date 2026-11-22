# research/AGENTS.md

The rules for researching an agent and for the research documents under
`research/`: how catalog entries are validated from source, the outline
every document follows, and how a research round is run with subagents.
The rules every change keeps are in the [root `AGENTS.md`](../AGENTS.md);
the catalog lines written from a document follow
[`internal/catalog/AGENTS.md`](../internal/catalog/AGENTS.md), a parser
follows [`internal/parser/AGENTS.md`](../internal/parser/AGENTS.md), and a
real install follows [`lab/AGENTS.md`](../lab/AGENTS.md).

## Validating catalog entries from source

Repeat this process when a tool is added or its layout may have changed,
before any install.

1. **Group the tools and fan out.** One researcher per platform group, in
   parallel. Forks of one codebase share a group (Cline with Roo Code and
   Kilo Code); closed-source tools get their own groups.
2. **Give each researcher the same brief:** the tool's current catalog
   lines (from `doubleagent catalog`), the repository to shallow-clone into
   the scratchpad, the grep terms, and the instruction to cite file and line
   for every claim as a link at the reviewed commit
   (`https://github.com/<owner>/<repo>/blob/<full sha>/<path>#L<n>`), never
   from memory. Grep terms that find the paths module quickly:

   ```text
   homedir  XDG  APPDATA  LOCALAPPDATA  "Application Support"  .config/  .local/share
   .local/state  .cache  sessions  history  checkpoint  snapshot  worktree  sqlite  .db
   auth  token  credential  secrets  keyring  keychain  cache  bin  node_modules
   cwd  workspace  project  worktree  mcp  rules  AGENTS.md
   ```

   Likely file names: `paths.ts`, `paths.rs`, `global.ts`, `storage.ts`,
   `home.go`, `config/load.go`, `env.ts`, `directories.rs`.
3. **Require a fixed report structure:** the outline of
   [Research document structure](#research-document-structure), under about
   a thousand words, plus a "Proposed lines" section in the catalog's
   formats (`agent|glob` for `CATALOG`, globs for `EXCLUDES` and
   `SECRET_GLOBS`, `project|glob` for `PROJECT_CATALOG`), dropped when the
   lines land. The build named in an Observed bullet is the version the
   evidence rests on; an install of another build that differs from it means
   a new round.
4. **Evidence order.** Best first; the report names each claim's level, and
   the evidence column of the index table in
   [`research/README.md`](README.md) records it.
   1. A real install: a per-step round
      ([Real-install rounds](../lab/AGENTS.md#real-install-rounds)) on
      Windows, macOS and Linux.
   2. Source code at a named commit, whatever its licence says about reuse,
      read with the project's documentation, each claim linking its line.
   3. For an agent without public source: first the vendor's documentation
      site and official public repository, cited at a commit like source
      unless its licence or terms expressly forbid the use. Then the
      shipped package or binary (`npm pack`, then grep or `strings`),
      always read, for key names, record shapes, path strings, enum values
      and log templates; a bundle string the documentation does not explain
      is recorded with the closest page and marked unexplained. Then vendor
      forums and issue trackers, then published DFIR or reverse-engineering
      write-ups.
   4. A vendor's own statement counts as documentation only where each
      claim points at something checkable (a documentation page, a release
      note, a bundle string, a path seen on a real install). An uncited
      claim is recorded in Source and evidence as a vendor statement with
      author and date until a round checks it. A vendor's pull request
      writes the research document, never the tables; the integration step
      and the round stay with the project.

   When sources disagree, a real install settles it; before one, the
   shipped package beats the page, and the document records both. Every
   agent gets a round; closed-source agents are queued first, by
   popularity, and an open-source agent is added from source with its round
   filed as an issue.

   A change written from a research document (table lines, discovery, a
   parser, a fixture) rechecks each claim it builds on against every source
   the document cites for it (a real install's Observed bullet, source code,
   a page, a bundle, a forum thread), one source at a time in the evidence
   order above; where they disagree, the document is corrected in the same
   commit, a real install settling it, then the shipped package. Before
   review, the change's review report (the subagent's report, or the commit
   body) lists each claim with the sources checked and the result, and
   answers each bullet of step 3 of
   [Adding a parser](../internal/parser/AGENTS.md#adding-a-parser) and each
   step of
   [Adding an agent to the catalog](../internal/catalog/AGENTS.md#adding-an-agent-to-the-catalog)
   the change touches, with the file and line that satisfies it or the
   reason it does not apply.

   A question about the product's surface (where a menu item lives, how a
   chat is deleted, what a command offers) is answered during a round first
   from the running agent's own help, completion lists and dialogs, saved as
   screens, and the research document is corrected from them. Outside a
   round, such a question or one about the product's behaviour (when a
   retention sweep runs) is answered first from the vendor's documentation
   and forum, found by web search or the documentation index (`llms.txt`)
   and cited as that source; a VM experiment then tests the documented
   claim. Grep the bundle only for key names, enum values, record shapes and
   path strings, or to confirm a string a page names.

   For minified bundles and compiled binaries:

   - Grep for a stable fragment of a message, never the whole sentence, as
     log lines are usually template literals
     (`${h?"Enabling":"Disabling"} remote control for session ${s}`); "not
     in the bundle" needs a second fragment to miss.
   - A compiled CLI or a bundle keeps its source strings at offsets that
     differ per platform build: cite `<tool> <version> <platform> @<byte
     offset>` with the literal snippet (`<file> @<byte offset>` for a file
     inside a bundle), the offset the 0-based one `grep -ob` prints for the
     cited string, the snippet the text around it, as
     [`research/claude-code.md`](claude-code.md) does for
     `claude 2.1.295 linux-arm64`. The artifact's size, sha256 and origin
     are recorded once, in Source and evidence, never in a citation. Where a
     table or section cites one build throughout, one sentence above it
     names the build and its files, and each citation gives only the file
     and offset with the snippet. A function or identifier name alone
     (`app.js function dot(e)`) is not a citation. The `app.asar` form is
     `app.asar:<path>@<offset>` with the snippet.
5. **Integrate in one commit.**
   - Research agents write only their research documents. One integration
     agent makes every table change after all reports are in, under the
     vendor's comment block (or a new one), never at the end, writing each
     line from the document's Collection sections, never from a report's
     Proposed lines.
   - It splits each group report into one document per catalog agent,
     named exactly as in the catalog, keeping citations and fixtures and
     dropping the proposed lines. Every agent with a catalog line gets its
     own document; [`research/frameworks.md`](frameworks.md) keeps only the
     sweep's summary table and the evidence for what got no line. Agents
     sharing a lineage each say so and link the other. Grouped reports are
     never committed.
   - The research README index lists every document, alphabetically by
     agent name, and holds the cross-agent observations.
   - It then makes the table, discovery and fixture changes of
     [Adding an agent to the catalog](../internal/catalog/AGENTS.md#adding-an-agent-to-the-catalog),
     runs `make check` and a live collection (reading the
     `skipped_excluded` and `secret: true` rows), and updates the research
     index table and the changelog.
6. **Close the loop on GitHub.** Comment on each tool's issue naming the
   evidence source and commit, the key findings and the catalog changes,
   and give it a `Closes #<n>` line in the pull request body. Comment on the
   agent's parser issue with any SQLite-only sources. File issues for
   anything out of scope the research surfaced.

## Research document structure

The research documents are the source of truth for what the project knows
about an agent: the catalog is written from the Collection part, the parser
from the Parsing part, and other pages and comments link the section
holding a fact rather than restating it. Every `research/<agent>.md` uses
the outline below, with the same heading names (they are the anchors), and
puts unique subsections under the section owning the fact.

A document covers one catalog agent, and a catalog agent is one state
directory. A surface (one form the product ships in) is its own catalog
agent and document when the catalog can attribute its state directory on
its own, at most through a nested entry (`copilot-cli`, `copilot-desktop`
and `copilot` are three). Surfaces writing one shared tree (the Cursor IDE
and its embedded agent CLI) are one agent and one document, with a `####`
subsection per surface under Per-user storage and under Transcript stores,
which says which surface writes each store, and one Observed bullet per cell
(one surface on one platform). A surface is never split out to shorten a
document nor merged in to save a catalog line.

`tools/check_research_structure.py` enforces the outline, the split phrases
and the placement of confidence paragraphs, and reports breaches of the
Observed bullet rules (label, build and earlier-builds sentence) and the
version, duplication and documentation-link rules below against its
baseline ([Quality gates](../AGENTS.md#quality-gates)); with `--before <dir>`
holding the previous copies it lists every citation, code span, fenced line,
number, label, Observed label and key-table row a rewrite dropped.

The document opens with `# <agent>` (the catalog name) and one unheaded
intro paragraph: what the agent is, its surfaces, its lineage and the
parser's state in one sentence. Nothing else precedes
`## Source and evidence`. A bold label never stands in for a heading; prose
wraps at 100 characters, except a citation link or a lone code span; tables
stay tables. Every reference to vendor documentation, a changelog, release
notes or a README is an inline link at the claim, to the section anchor when
the page has one.

1. `## Source and evidence`: the repository and commit (or bundle, binary,
   documentation), the evidence level per claim, then `### Observed` and
   `### Source reviews`. When the reviewed commit is ahead of the latest
   release tag in what it writes, the document names that tag beside the
   commit, and Format versions says which shapes the tag's build writes
   differently, since a host runs a released build.
   - An Observed bullet is one cell, one platform and surface: build, date,
     the steps run and what each wrote, in one paragraph. It opens with its
     cell label in bold, `**<Platform> <surface>**:` (the colon inside or
     after the bold), where Platform is one of macOS, Windows, Linux,
     FreeBSD or OpenBSD and surface names the product's form
     (`**macOS CLI**:`, `**Windows desktop app**:`,
     `**Linux lab container**:`); a document with no install has the one
     bullet or paragraph opening `No real install.` or `No real install:`,
     followed by the evidence that stands in.
   - Each cell has one bullet, which describes the latest round of its cell,
     names that build, and, when earlier builds were observed, ends with
     `Earlier builds observed: <build> (<YYYY-MM-DD>), <build> (<YYYY-MM-DD>).`;
     what an older build wrote differently goes in Format versions. Its
     findings are stated in the sections that own them and linked from the
     bullet.
   - What `doubleagent` did during a round goes to the docs page it
     concerns or to an issue, never into the document.
   - A Source reviews entry is the date, the commit or build reviewed, and
     one or two sentences on what changed in the document, newest first;
     "None." when none. Its facts go into the owning sections, which never
     cite the entry.
2. `## Collection`, the half `collect` is written from:
   - `### Per-user storage`: every directory and file under the home per
     platform, what each holds, sizes where observed; `####` subsections
     per platform or product when layouts differ; the fixed
     `#### Scratch directories` ("None known." when none) and
     `#### Relocation variables` after it. The latter is a table with the
     header `| Variable | Moves | Replaces | Table agent |`: Variable a code
     span, Moves what it relocates with its source citation, Replaces the
     home-relative path the value stands in for (`.` when the value stands
     in for the home itself as the parent of the agent's directory, empty
     when none), Table agent the `RELOCATION_VARS` agent, `shared` for a
     generic variable; an agent reading none has the subsection say "None
     known.". A Moves cell that calls the value the parent of the directory
     Replaces names is a `tools/check_tables_research.py` finding.
     `research/shared.md` holds the `shared` rows and a
     `#### Startup and environment files` subsection naming each
     `STARTUP_FILES` glob in a code span. A Docker volume layout is
     described in a Per-user storage subsection whose heading or opening
     paragraph says "volume" and names the volume in a code span (its name,
     its `DOCKER_VOLUMES` glob or its catalog line), and names each
     volume-relative exclusion and secret glob. Each table line of the agent
     is named in the Collection part as its glob, a path it matches or (a
     home or project line) lies above, its remainder below one of the
     agent's own catalog entries, or, for a glob opening `*/`, its remainder
     below the entry it reaches (`tools/check_tables_research.py` checks
     this).
   - `### Credentials`: credential files (settings files that hold keys
     among them), keychain use and its file fallback, identity values to
     redact. A credential file is named by its home-relative path on each
     platform form the Per-user storage section names, and each is either
     flagged by a secret glob or, for a store that mixes secrets with
     evidence, listed in Secrets; "collected unflagged", "left unflagged",
     "not flagged", "may embed" and "can embed" are not used.
   - `### Exclusions`: large or low-value subtrees as exact patterns.
   - `### Project-local files`: files written or read inside repositories.
   - `### Where the project path is recorded`: file and key, or table and
     column, that project discovery or the parser reads.
3. `## Parsing`, the half the parser is written from; an agent with nothing
   to parse has one paragraph here (collected only, or the parser issue)
   and no `###` sections:
   - `### Transcript stores`: where transcripts live and how they are
     named, per surface; the fixed `#### What survives a delete`: what the
     agent's own delete, clear, archive or retention removes, what remains
     (the history file, second copies, deletion markers, copies held by
     other agents, project-local traces) and what the parser reads, with a
     link to
     [Secondary sources](../docs/timeline-output.md#secondary-sources) and
     no `doubleagent` version.
   - `### Record schema`: line or row types and their key sets, `####` per
     store or record family; the field names the parser depends on. A key
     table, here or under Other stores, has the header
     `| Key | Type | Values | Since |` (or `Builds` for `Since`): Key a code
     span, Type the JSON type names (`string`, `number`, `boolean`,
     `object`, `array`, `null`), and `blob` for a SQLite BLOB column, joined
     by "or", a comma or a slash, Values the allowed values as code spans,
     empty for any value.
   - `### Joins and timestamps`: session id, project path, tool-call ids,
     the time basis and precision, what cannot be joined.
   - `### Other stores`: databases, indexes, histories, logs, mirrors and
     backups beside the transcript store, `####` per store.
   - `### Format versions`: how the shapes changed and which build writes
     which. Elsewhere a shape description names a version only as
     `since <version>` or `added in <version>`, backed by a Format versions
     entry naming that version or a citation link on the same line or the
     next; an observed change without a commit or changelog is written
     `changed between <build> and <build>`.
   - `### Secrets`: what inside the stores is a credential or an identity.
   - `### Parser plan`: the row table and the globs, then the fixed `####`
     subsections in this order, each present and saying "The agent records
     none." or "Not stated." when that is the fact: `Permission rows`,
     `Model rows`, `Past-use rows and secondary sources`,
     `Sync, relay and origin rows`, `Scratch directory rows`,
     `Deletion rows`, `Bad-line rows`, `Fixtures`; unique subsections follow
     them. A list of what the parser does not read starts `Not read:` and
     names each file or key as a code span; a parenthesis inside it that
     says something is read is phrased ``(`x` is read only for ...)``, and a
     clause opening `whose` (or `its` or `their` after a comma) describes
     the item before it and names nothing up to the next comma, semicolon,
     colon or full stop. A Fixtures sample is a record as an install wrote
     it, redacted.
4. `## Confidence`: one list for both halves, confidence per item and what
   could not be determined, holding no fact the body states and no item it
   has answered. A confidence paragraph elsewhere is a defect.

One home per fact: Per-user storage names each file and what it holds;
Transcript stores through Other stores describe its records; Credentials
names credential files and Secrets the stored fields that are credentials;
other sections link the home. A fact stated in two sections is a defect
even when the copies agree: each record shape is described once, and a
version is named only where the shape changed. The document speaks as one:
it never refers to "the collector document", "the analyzer review", "the
two documents" or a "collector concern"; a dated subsection ("Found in the
review of ...") is folded into its topic; a section link points at the
topic it names.

`research/README.md` is the index (evidence level and parse status per
agent, and the survival table); `research/frameworks.md` (one section per
framework) keeps its own shape. This file and its `CLAUDE.md` are not
research documents, and the checkers skip them.

### Research rounds with subagents

The split in the [root `CLAUDE.md`](../CLAUDE.md) applies: the main
session writes one brief and a per-tool assignment, one research subagent
per tool runs in parallel, one integration subagent makes the table, test
and index changes, and the main session reviews. The brief names the
`AGENTS.md` files each subagent reads (this one, and
[`internal/catalog/AGENTS.md`](../internal/catalog/AGENTS.md) for the
integration) and sets the safety envelope:

- Fetch untrusted code; never execute it. Clones, `npm pack` tarballs and
  binaries for `strings` go in the scratchpad and are only read: no
  `npm install`, no installer, no running the tool. A real-install round in
  a lab VM is a separate step the user asks for explicitly.
- A blocked clone or download is reported, never worked around from
  memory.
- Nobody runs `doubleagent collect` on the workstation by accident: the
  root spellings that give a live run or read the whole workstation are
  listed under [Tests](../internal/AGENTS.md#tests). A reviewer trying root
  spellings uses a root inside a scratch image, and a brief for a subagent
  says so.
- A subagent that runs a live collection against the workstation writes it
  under the scratchpad, deletes it afterwards, and reports no file contents
  or manifest rows from it; or the main session runs it.
- The subagent reports what to post to GitHub; the main session posts it.

The review gate is mechanical first: run `tools/check_research_links.py`
(with `--repos` pointing at the scratchpad checkouts when they exist) and
treat any unresolved link, and any claim without a link, as unverified.
Then the main session repeats the integration subagent's checks: the drift
checkers, `make check` and the catalog coverage test.

### Research checks

- A library with no platform branching (`xdg-basedir`, `etcetera`,
  `home.Config()`) means `~/.config` and `~/.local/share` on macOS and
  Windows too.
- A variable that moves a store is a `RELOCATION_VARS` line, found in the
  agent's paths module or documentation with the grep terms above plus
  `process.env`, `os.environ`, `getenv` and `env::var`; a path read from the
  environment is never a `CATALOG` line.
- Exclusions must not swallow credential files (Continue's
  `index/globalContext.json` sits beside the embedding index).
- A tool may store sessions inside the repository or gitignore its state
  directory: the project list, not git, is the index.
- Auth tokens and chat history can share one SQLite file; collect it
  unflagged and document the keys to redact.
- Forks drift and repositories move or stop being the product: confirm
  what the current commit ships before reading paths, and follow a move.
- A wrapper around another agent needs that agent in the catalog;
  researchers report every other agent whose state directory their tool
  reads or writes.
- Entries differing only by case (`.config/PearAI` and `.config/pearai`)
  name one directory on a case-insensitive file system, which is collected
  once: add both only when distinct on Linux, and make their fixture test
  attribution, not count.
- The integration agent derives the number of agents from the documents in
  the tree, not the brief, and says when they differ.
- Chat history outside the home cannot be a catalog line: record it in
  [`docs/coverage.md`](../docs/coverage.md) as a manual step and file an
  issue. Docker and Podman named volumes are the exception: they go in
  `DOCKER_VOLUMES`.
