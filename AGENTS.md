# AGENTS.md

Guidance for AI coding agents and humans working in this repository.

## Purpose

`doubleagent` is a forensic tool for incident responders. Its collectors
archive the on-disk artifacts of AI agents for every user on a host, with a
hashed manifest; its analyzer, the `doubleagent` Python command, detects
agent state in a collection or any directory tree and parses transcripts
into a JSONL timeline. The catalog takes any agent, editor or not, that
executes tools or shell commands on the host and leaves transcripts,
configuration or credentials on disk. When a new kind of agent is added,
widen the wording in the [top-level README](README.md) in the same commit.

The project supplements the responder's EDR and never duplicates it:
process lists, logged-on users, network connections, services, scheduled
tasks, shell histories and any other feature an EDR console provides are
out of scope.

The sh and PowerShell 5.1 collectors run on the host and must work through
an EDR remote shell, locally by a responder, and against a mounted disk
image on an analyst workstation, including Windows images mounted on Linux
or macOS. The analyzer (`analyzer/`) runs on the analyst's workstation and
works without the collectors; parsing never happens on the host. The
[agents page](docs/agents.md) lists every catalog agent with its parse
status.

## Repository layout

```text
collectors/
  collect-agent-artifacts.sh   the macOS/Linux/BSD collector, single POSIX sh file
  Collect-AgentArtifacts.ps1   the Windows collector, single PowerShell 5.1 file
  PSScriptAnalyzerSettings.psd1  PSScriptAnalyzer rules, including the 5.1 compatibility
                               profiles
  README.md                    collector user documentation, keep in sync with behaviour
  docs/<topic>.md              detailed collector documentation: options, output and
                               manifest schema, coverage, Docker, privileges, inventory,
                               Windows, deployment, testing, internals
  CHANGELOG.md                 collector release history, one entry per VERSION
  tests/smoke.sh               end-to-end test of the sh collector (fake disk image)
  tests/smoke.ps1              end-to-end test of the PowerShell collector
  tests/catalog-sync.sh        fails if the catalog tables differ between the scripts
                               or from the analyzer's bundled copy
analyzer/
  doubleagent/                 Python package: cli, inputs, catalog, model, parsers/
  doubleagent/catalog.txt      verbatim copy of collect-agent-artifacts.sh --list
  README.md                    analyzer user documentation: quick start, commands, options
  docs/<topic>.md              detailed analyzer documentation: inputs and detection,
                               output and JSONL schema, filtering, searching, inventory,
                               options, development
  CHANGELOG.md                 analyzer release history, one entry per VERSION
  tests/                       unittest suite with synthetic fixtures; tests/run.sh
  pyproject.toml               installable as the doubleagent command
  requirements.txt             third-party dependencies (python-dateutil;
                               zstandard, for compressed transcripts)
research/<agent>.md            everything known about one agent, one per agent: the
                               Collection part (paths, credentials, exclusions, project
                               files, discovery) and the Parsing part (how it records
                               transcripts, the parser plan)
research/README.md             the research index and the survival table
research/frameworks.md         the framework sweep
lab/                           Docker sandbox for running CLI agents against a scratch
                               home and exporting it for the collector; see lab/README.md
docs/index.md                  the documentation site's home page; the root README.md is
                               the GitHub overview and is not a site page
docs/agents.md                 every catalog agent with its parse status, files and evidence
docs/lab-vms.md                the lab VMs: the development tools in their clean snapshots,
                               the commands that rebuild them, Docker's requirements per VM
tools/check_research_links.py  verifies every citation link in the research documents
                               resolves to its file and line range at the cited commit
tools/check_research_structure.py  verifies every research document follows the
                               outline; with --before, lists facts a rewrite dropped
tools/check_doc_links.py       verifies every relative link and heading fragment in the
                               Markdown documents, that section and external
                               references are links, that a reader page
                               introduces this file's terms of art before using them,
                               and that this file and CLAUDE.md link no issue or pull
                               request
mkdocs/mkdocs.yml              the documentation site: MkDocs with the Material theme,
                               built from the documents where they are; its nav: tree
mkdocs/hooks.py                site build hooks: GitHub heading ids, code spans with
                               end spaces, links to non-page files, README URLs
requirements-docs.txt          the site's Python dependencies, pinned
README.md                      project overview; points at the per-part READMEs
AGENTS.md                      this file
CLAUDE.md                      imports this file for Claude Code
LICENSE                        Apache 2.0
ruff.toml, pyrightconfig.json, .markdownlint-cli2.jsonc, .shellcheckrc
                               linter configuration; see "Quality gates"
.githooks/pre-commit           the catalog drift check
.github/                       CI workflow, the Pages workflow (workflows/pages.yml,
                               publishes the site) and Dependabot configuration
```

Nothing in `collectors/` may depend on `analyzer/`. Paths in this file are
relative to the repository root unless stated otherwise.

## Non-negotiable standards

**No dependencies on the host.** The collector uses only sh, find, cp, tar,
gzip, du, awk, grep, sed, sort, cut, tr, readlink, mkdir, rm, and one of
`sha256sum`, `shasum`, `sha256` or `openssl`; never Python, Perl, jq or
anything a minimal host may lack. The Windows collector is PowerShell 5.1
with no modules.

**POSIX sh, not bash.** The shebang is `#!/bin/sh`, and the script runs
unchanged under bash 3.2, dash, busybox ash, FreeBSD and OpenBSD sh, posh,
and `zsh --emulate sh`.

- No arrays, `local`, `[[ ]]`, `read -d`, `mapfile`, `${var,,}`,
  `${var:0:3}`, `$'...'`, `function` keyword, `-print0 | read -d ''` or
  process substitution.
- No GNU-only flags (`sed -i`, `date -d`, `cp --parents`, `find -false`);
  `stat -c` and `xargs -0` only with a BSD fallback.
- No GNU basic-regex extensions (`\|`, `\+`, `\?`, `\b`, `\w`) in `grep` or
  `sed`; use `grep -E` or two `-e` patterns. Only the macOS CI job catches
  these.
- Platform differences (`stat`, hashing, user enumeration) go behind a
  wrapper function with capability detection once at startup.
- Shared helpers live in the `COMMON` string, `eval`'d in the main script
  and prepended to the worker programs, since `find -exec sh -c` does not
  inherit functions.
- Expand globs with spaces with `IFS` set to newline only (`expand_glob`);
  match patterns with `case`.
- pdksh-derived shells end a `$(...)` at the first `)`, even a `case`
  pattern's inside a here-document. Write `case` patterns inside command
  substitutions as `(pattern)`, and read a here-document into a variable
  with `heredoc_var`, not `VAR=$(cat <<'EOF' ...)`.
- Every sh file passes shellcheck, checkbashisms and the shfmt POSIX parse
  ([Quality gates](#quality-gates)).

**Windows PowerShell 5.1, not PowerShell 7.** `Collect-AgentArtifacts.ps1`
runs under the PowerShell of Windows 10, 11 and Server 2016+, and under
PowerShell 7 on any OS.

- No ternary operator, `??`, `-AsHashtable`, three-argument `Join-Path`,
  `[System.IO.Path]::GetRelativePath` or `.NET Core`-only APIs;
  `Get-FileHash`, `ConvertTo-Json -Compress`, `[DateTimeOffset]` and
  `System.IO.Compression.FileSystem` are fine.
- No variable named after an automatic variable (`$home`, `$host`, `$args`,
  `$input`, `$error`, `$pid`, `$profile`); use `$homeDir`, `$HostName`,
  `$cargs`.
- Under `Set-StrictMode -Version 2` on 5.1 a scalar has no `.Count` and a
  missing property throws: wrap a possible single item in `@()`.
- Splat named parameters to a script file with a hashtable, not an array.
- Use `-Force` on `Get-Item` and `Get-ChildItem` for hidden items
  (dotfiles under PowerShell 7 on Linux, `AppData`).
- Use `tar.exe` only when `$env:OS` is `Windows_NT`, `ZipFile` elsewhere.
- Write files with `StreamWriter` as UTF-8 without BOM, not `Add-Content`.
- Open source files with `FileShare.ReadWrite | Delete`.

**Single file, non-interactive.** No prompts, nothing read from stdin, all
output under `-o`, concise stdout, exit `0` when an archive was written
even if some files failed.

**Forensic soundness.** Never modify, move or delete anything outside the
output directory. `lstat` every file before copying it, so the manifest
holds pre-copy times. Hash the staged copy, not the source. Never follow
symlinks; record each with its target and recreate it. Record everything
skipped (size exclusions, oversized files, secrets) in the manifest with a
status and size. Collect credential files by default, flag them
`secret: true`, and honour `--no-secrets`. Never print secrets or file
contents to stdout or the log.

**Catalog is data, not code.** Agent paths live in the `CATALOG`,
`PROJECT_CATALOG`, `EXCLUDES`, `SECRET_GLOBS` and `DOCKER_VOLUMES` tables
of both scripts; a new tool is new lines, never new code. Every entry is
tried against every home directory.

- The tables are byte-identical between the sh single-quoted strings and
  the PowerShell `@'...'@` here-strings, and `--list` equals `-List`;
  `collectors/tests/catalog-sync.sh` checks both.
- `DOCKER_VOLUMES` maps a Docker or Podman volume name glob to an agent
  (`agent-zero|*a0_usr`); the first match wins. A matched volume is
  collected whole, its `_data` directory the collection base; an unmatched
  one gets one `skipped_unmatched_volume` row. A tool in a volume adds
  volume-relative exclusions and secret globs (`tmp/playwright`,
  `secrets.env`) at the end of each table, never colliding with another
  tool's volume layout (no bare `tmp` or `cache`).
- Catalog globs do not cross `/`; exclusion and secret globs do, match
  relative to the collection base (a home, a discovered project or a
  volume's `_data`), and never start with `*/`. `[...]` and `[!...]`
  classes work in both collectors.
- Anchor every glob on a path component (`.hermes*/auth.json`, never
  `*hermes*/auth.json`); the one accepted leading `*` is
  `*/User/globalStorage/...`.
- A secret glob may contain spaces but never `|`, `(`, `)`, `&`, `;`, `<`,
  `>`, `$`, a backtick, a quote or a backslash, and never starts with `#`,
  because the sh collector compiles the table into one `case` pattern
  (`compile_secret_pattern`); `catalog-sync.sh` checks.
- When an exclusion and a secret glob both match, the exclusion wins and
  the directory's `skipped_excluded` is the subtree's only row; a
  credential inside needs a narrower exclusion (`hermes-agent/[!.]*` keeps
  `.env`).
- PowerShell matches case-insensitively. A glob differing from a sibling
  only by case (`*/Hermes/Cache` beside `hermes/cache`) names explicit
  bases instead. When an exclusion ending in a common name
  (`*/Claude*/Cache`) also prunes another tool's directory, that tool's
  entry names the directory's children
  (`AppData/Local/claude-cli-nodejs/Cache/*`), never the directory, and the
  PowerShell smoke test checks a file under it.
- A nested entry is an entry inside another entry's match
  (`antigravity|.gemini/antigravity-cli` inside `gemini-cli|.gemini`). It
  claims its subtree, so both collectors and the analyzer's loose mode
  attribute each file once, to the most specific agent. Use one whenever a
  tool stores state inside another tool's directory.

**Manifest schema is an interface.** The fields, status values
(`collected`, `symlink`, `skipped_excluded`, `skipped_size`,
`skipped_secret`, `skipped_special`, `error_copy`,
`skipped_unmatched_volume`, `error_read`) and `fs/<original path>` archive
layout of `manifest.jsonl` and `collection.json` are documented in
[`collectors/docs/output.md`](collectors/docs/output.md). Renaming or
removing one needs that page updated and a note in the commit message.

### Readability

Both collectors are written for a responder to audit before running them
as root; [`collectors/docs/internals.md`](collectors/docs/internals.md)
holds the architecture and function tables.

- **One layout.** Both scripts have twelve sections, in this order and
  under these banner titles: Reader's guide, Data tables, Shared helpers,
  Manifest writer, Staging files, Catalog walk, Users, Docker and Podman
  volumes, Project discovery, Inventory output, Summary and archive, Main
  program. A section one script lacks keeps its banner and a line saying
  why. The reader's guide lists the sections, the data flow in one
  paragraph and the naming rules, and changes with the flow. New code goes
  in the section owning its step; the main program is one call per step.
- **One comment per function**, directly above it: its inputs (in sh, also
  the globals it reads), what it returns, prints or writes, in sh its
  temporaries' prefix, and any quirk it exists for.
- **sh names.** Shared globals are `UPPER_CASE`; a function's temporaries
  `_<prefix>_<name>`, the prefix abbreviating its name (`_cp_` for
  `collect_path`); a function that assigns its result sets `_ret_<what>`
  (`_ret_json`). A worker program is held in `<NAME>_WORKER`, read from the
  here-document `EOF_<NAME>_WORKER`, its first line naming its positional
  arguments; other here-document labels name what they feed
  (`EOF_USER_LIST`).
- **PowerShell names.** Script state is `$script:PascalCase`, parameters
  and locals `$camelCase`. Functions use approved verbs; one that sets up
  or changes run state uses `Initialize`, `Open`, `Close`, `Switch` or
  `Clear`, never `New`, `Set`, `Remove`, `Start`, `Stop` or `Update`.
- **Both.** No single-letter or cryptic names except a short-lived loop
  index; a variable that fills a manifest field is named like it (`mtime`,
  `archive_path`, `sha256`).
- **One name per job.** The sh and PowerShell functions for one step have
  matching names (`collect_path`, `Add-CollectedPath`) and one row in the
  [function pairs table](collectors/docs/internals.md#function-pairs),
  updated in the same commit as any function added, renamed or removed on
  either side.
- **Bounds.** A function body stays under 40 lines, never over 60; over 40
  only for a per-item loop that a split would give an extra call per file
  or member (`Add-CollectedFile`, `Write-TarEntries`), a flat list of
  discovery sources (`Find-AgentSessionProjects`) or one literal
  (`Initialize-Summary`). Lines outside the data tables are at most
  100 characters, broken in sh with `\` or `set --`, in PowerShell with
  splatting or variables, never a backtick. No new fork, pipeline or
  external command per file in the sh workers or `Add-CollectedFile`.
- **Acceptance test for a refactor.** A readability change changes no
  behaviour. Run the collector before and after on the smoke test's fake
  image, for every run the smoke tests make with the same arguments, under
  every shell of the matrix and PowerShell 7 and 5.1, and compare options,
  `--list`, manifest rows (every field, value and the row order),
  `collection.json`, log lines and their order, stdout, stderr, exit codes,
  archive members, the tables byte for byte, and the `COLLECTOR_SH` and
  `CAC_*` environment variables; only `atime`, the `started`, `finished`
  and `archive` keys, timestamps, the output directory and the archive's
  size and hash may differ. A bug found on the way is fixed in its own
  commit, with an issue.

## Analyzer standards

The analyzer runs on the analyst's workstation, never on the host.

- **Python 3.10 or later, standard library first.** A third-party package
  only when a format needs it, listed in both `requirements.txt` and
  `pyproject.toml`. No syntax newer than 3.10.
- **The catalog is shared, not copied by hand.** `doubleagent/catalog.txt`
  is generated from `collect-agent-artifacts.sh --list`; regenerate it after
  any catalog edit. `collectors/tests/catalog-sync.sh`, the analyzer tests,
  CI and the pre-commit hook ([Quality gates](#quality-gates)) fail on a
  stale copy. Detection lives in `catalog.py`. Parsers select files by
  `artifact.rel`, the path relative to the home, never by a hard-coded
  agent path.
- **Attribution comes from the manifest when there is one.** Host, user,
  home and agent come from `collection.json` and `manifest.jsonl`; only
  loose input without a manifest infers them from paths, and the output
  says so. A row attributed to a project or session takes each value from
  that record alone, never a sibling's, and omits a value it lacks.
- **Parsers are validated like catalog entries.** Confirm the record shape
  against source or a real install, record it in the Parsing part of
  `research/<agent>.md`, list the fields the parser depends on in the
  module docstring, and build the fixture from those shapes. Extract a
  closed-source binary's protobuf descriptors with
  `analyzer/tools/proto_descriptors.py` and cite their proto file in the
  parser's schema table. Decode protobuf with `doubleagent/protobuf.py`;
  open SQLite only through `doubleagent/sqlite_util.py`, which reads WAL
  sidecars and never opens the evidence copy directly.
- **Bad lines are evidence.** A bad line is never fatal; every turn before
  and after it is still yielded. Each line that does not decode becomes its
  own `system` row `parser: unparseable line (<reason>): <line>`, holding
  the line as read less its terminator, unrepaired and unshortened; a JSON
  array reader writes `unparseable record <N> (<reason>)`. A whole-file
  error (unreadable, not a JSON array, cut inside an array) is one row
  `parser: <message>`. The row carries the file's known `session_id` and
  `project_path`, an empty `timestamp_utc` and `source_line` set to the
  line. This covers every file a parser reads: history files, indexes and
  secondary sources (files that repeat the primary store's events; see
  [Secondary sources](analyzer/docs/output.md#secondary-sources)).
- **Problems outside parsers.** A file the analyzer command reads itself (a
  repository's config and HEAD files, an index, a settings file) that fails
  to read or parse gives a `problem:` line, never an empty result, reported
  to stdout and the JSON outputs in the same run, in the order
  [`analyzer/docs/output.md`](analyzer/docs/output.md) documents.
- **Bad-line mechanics.** A reader appends `(line, message, raw)` to an
  `errors` list (`iter_jsonl` and `line_text` in `parsers/base.py` do so);
  the parser ends each file with
  `yield from error_rows(self, artifact, errors, session_id, project_path)`.
  No parser formats a bad-line row itself; only a whole-file row is written
  in place. Every filled `errors` list is reported, returned or passed on;
  only a pre-scan of a file the same parser also parses in full may pass a
  literal `[]`. `ErrorListsReportedTests` in `analyzer/tests/test_errors.py`
  enforces this.
- **Validation evidence.** The agent's subsection under
  [Parsed agents](docs/agents.md#parsed-agents) names the real installs
  (platform, version, date) behind the parser's paths and shapes, and says
  "fixture only" for a path resting on source or a bundle alone. The
  research document holds one Observed bullet
  (the record of one real install of one form of the product, such as its
  CLI, on one platform; see
  [Research document structure](#research-document-structure)) per
  platform.
- **The outputs are interfaces.** The `timeline.jsonl` and `sessions.jsonl`
  fields, order and JSON types are documented in
  [`analyzer/docs/output.md`](analyzer/docs/output.md), the
  `fleet-inventory.csv` columns in
  [`analyzer/docs/inventory.md`](analyzer/docs/inventory.md). Add fields or
  columns only at the end; rename, retype or remove one only with those
  pages updated and the commit saying so. Every JSONL record has every
  field, `""` rather than `null` for no value. Timestamps are UTC ISO 8601
  with milliseconds and a trailing Z.
- **Read-only.** The analyzer writes only under `-o` and the archive work
  directory, and never follows or materialises symlinks. The `text` field
  holds the full event text, line breaks included, never truncated.
- **Readers follow the specification.** A reader for a specified format (a
  repository config file, an INI dialect, a TOML or YAML subset) is written
  from the specification, with one test per rule: quoting, escapes,
  continuation, comments, header forms, encoding and case. Lines end at LF
  or CRLF only (not `str.splitlines()`), trims are ASCII only (not
  `str.strip()`), and an untrusted evidence file is read whole, uncapped,
  in linear time.
- **Lint and types.** ruff and pyright in standard mode are clean
  ([Quality gates](#quality-gates)); fix a finding rather than suppress it,
  and give any `# noqa` or `# pyright: ignore` a reason.
- **Tests.** `analyzer/tests/run.sh` passes. Every parser has a fixture and
  a test class, every new input shape a detection test. A proximity join of
  two record kinds is bounded by the next record of the same kind and
  tested with two interleaved pairs (A, A's partner, B, B's partner). A
  test needing POSIX permissions or an unprivileged user skips with the
  `CAN_DENY` guard of `analyzer/tests/test_errors.py`.

## Adding a parser

The research is done for every agent in the parser backlog; do not redo
it. Start from the Parsing part of `research/<agent>.md`, whose Parser plan
maps records to timeline columns and whose Fixtures subsection holds the
sample records. Check the record shape at the cited commit, not the default
branch, and note any drift in the module docstring.

A per-step round (an install in a clean VM with a listing after every user
action; [Real-install rounds](#real-install-rounds)) comes first for an
agent whose research rests on source alone, a newer agent version, and a
surface (one form the product ships in: a CLI, a desktop app, an editor
fork, an extension, a packaged build beside a direct download) not yet
exercised. Its line types and key sets go into the Record schema and Parser
plan, and the parser is written from them.

1. Create `doubleagent/parsers/<agent>.py` with a subclass of `Parser` from
   `parsers/base.py`.
   - Its `agent` attribute equals the catalog name. `wants()` selects files
     by `artifact.rel` with a regex, and `secondary()` the same way the
     files that repeat the primary store's events. `parse()` yields rows
     from `base_row()`, which fills host, user, agent and `source_file`.
   - Reuse `iter_jsonl`, `text_of` and `compact_json` from `base.py`,
     `compact` from `model.py` and `to_utc` from `timeutil.py`. List the
     fields the parser depends on in the module docstring.
   - A parser reading a file the catalog attributes to another agent lists
     that agent in `reads_agents`; nothing else routes files across agents.
     A wrapper over another agent's format reuses that agent's reader.
   - A helper two parsers need lives in `parsers/base.py` or its own
     module, never in a parser module. A parser imports another parser
     module only to subclass it or reuse its reader, one way.
2. Register an instance in the `ALL` list in `parsers/__init__.py`; nothing
   else discovers parsers.
3. Follow the shared row semantics.
   - `tool_use` rows carry `tool_name` and `tool_use_id`; `tool_result`
     rows join on `tool_use_id`; `thinking` rows only when
     `opts.include_thinking` is set.
   - `session_id` and `project_path` propagate from the session header to
     every row; `git_branch` stays empty when the format lacks it. A format
     without per-message timestamps inherits the session timestamp, and the
     docstring says so.
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
     histories. `secondary()` is True for those that repeat the
     primary store's events, so `dedup.deduplicate` keeps one copy of each
     event and what only the secondary source holds
     ([Secondary sources in `analyzer/docs/output.md`](analyzer/docs/output.md#secondary-sources)).
     Such a parser sets `has_secondary`, so recovered rows and sessions are
     marked `recovered` in `timeline.jsonl`, `sessions.jsonl` and the
     inventory. A secondary source without its own record times gives rows
     an empty `timestamp_utc`, never the session's time; one that stamps
     events later than the primary store returns a wider window from
     `secondary_tolerance()` and says why in its docstring.
   - **Sync, relay and origin rows.** A marker of a session uploaded to the
     vendor's account gets a `system` row
     (`session synced: remote session <id> last event <id>`, or the agent's
     own terms) at the file's modification time, saying so, and the parser
     sets `records_sync`. A session carried through a messaging platform is
     a relay: it gets a `session relayed:` row and the parser sets
     `records_relay`, so `sessions.jsonl` `relayed` reads `yes` or `no`. A
     session that originated on the vendor's servers and is only listed
     locally is synced and gets a `session origin: remote <service>` row;
     the parser sets `records_origin`, so `origin` reads `remote` or
     `local`.
   - **Source-host rows.** A recorded source-host account gives a `system`
     row `source host: <host> account <login>` at the record's time. A cached
     token is a credential (flagged, never printed); a repository list is a
     project list.
   - **Research first.** Before the parser is written, each fixed Parser
     plan subsection of the research document names the files and fields
     behind its rows, or says the agent records none. For the model it says
     where it is recorded per message, session and switch; for past use
     what each file holds, what it survives (a deleted session, a rewind, a
     removed project, a compaction), how it joins the primary store
     (session id, project path, text and time) and whether it is primary or
     secondary; for sync, the command or setting
     that syncs or unsyncs; for a source host, where the account, its token
     or grant (a file, secret storage, the OS keychain, or server-side
     only), the repository list and repository-bound conversations live.
   - **Survival table.** Update the agent's row in the
     [survival table of `research/README.md`](research/README.md#where-transcripts-live-and-what-survives)
     in the same commit: one short sentence per cell with the file name,
     the one qualifier that matters, `parsed`, `secondary` or `not parsed`,
     and a link to the document's `#### What survives a delete`, which
     holds the facts; a fact the table holds that the document lacks goes
     into the document first.
4. Open SQLite and decode protobuf as
   [Analyzer standards](#analyzer-standards) says; a new dependency goes in
   both `requirements.txt` and `pyproject.toml`.
5. Add a `<agent>_records()` builder to `tests/fixtures.py`, wired into
   `build_home`, using the `/srv/proj` project path, a noise file the
   parser must not want, and `write_bad_line` for the truncation case.
6. Add a `<Agent>Tests(ParserBase)` class to `tests/test_parsers.py` with
   the existing cases: rows, thinking opt-in, history, not-wanted,
   truncated line. The truncated-line case covers every file read line by
   line and checks that the lines around the bad one are yielded beside the
   `parser:` row. A proximity join gets the interleaved case.
7. In the [agents page](docs/agents.md) table, set the agent's status to
   `parsed` and its "Validated against" cell to the evidence class and
   platforms in a few words (`real installs: macOS, Windows (October 2026)`,
   `source; fixture only`); its Notes cell keeps the
   research link, the open issue links and, only while something is
   unparsed, a clause of about twelve words. Move its subsection from
   [Collected agents](docs/agents.md#collected-agents) to
   [Parsed agents](docs/agents.md#parsed-agents), alphabetically, with
   "Files read:" and "Validated against:" (platform, version and date of
   every real install). Move the agent out of every example showing it
   under `detected but not parsed`, `collected only` or `parser: false`.
   Bump `VERSION` in `doubleagent/__init__.py` and run
   `analyzer/tests/run.sh`.

The backlog may be delegated one parser per subagent, in parallel. A
subagent changes no assertion outside its own test class, and the merge
step re-derives the shared counts rather than taking either branch's.

## Adding an agent to the catalog

Prefer reading the tool's source over installing it. Decide what counts as
one agent first, by the rule under
[Research document structure](#research-document-structure); the name in
the `agent|path` lines, the document's file name and the parser's `agent`
attribute are the same string.

1. Shallow-clone the repository into the scratchpad, grep for the terms
   under
   [Validating catalog entries from source](#validating-catalog-entries-from-source),
   read the paths module, and cite file and line in the issue. For a
   closed-source tool, follow the evidence order there (documentation, then
   the shipped bundle or binary, then forums and write-ups) and say which
   level each claim came from.
2. Identify transcripts, history, config, credentials and anything large
   (binaries, model weights, caches, indexes, shadow git checkpoints,
   worktrees, marketplace clones).
   Many tools use `~/.config` and `~/.local/share` on macOS and Windows
   too, and several keep credentials in the OS keychain with a file
   fallback only on headless hosts.
3. Add `agent|path` lines to `CATALOG` in both scripts for every platform
   path, including the Windows `AppData` location. Regenerate
   `analyzer/doubleagent/catalog.txt`, run `collectors/tests/catalog-sync.sh`,
   and add an `Added` line under `Unreleased` in
   [`analyzer/CHANGELOG.md`](analyzer/CHANGELOG.md).
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
   - A named Docker volume the tool's README, compose file or installer
     creates goes in `DOCKER_VOLUMES`, its source cited in the comment
     above it.
5. If the tool records project paths, add extraction to the matching
   source group of `discover_projects` (sh) and `Find-Projects`
   (PowerShell), at the same position on both sides
   ([project discovery groups](collectors/docs/internals.md#project-discovery)),
   and any per-project files to `PROJECT_CATALOG`. Process names are never
   listed. The analyzer, not discovery, reads project paths that live only
   in SQLite.
6. Add a fixture and checks to `collectors/tests/smoke.sh` and
   `collectors/tests/smoke.ps1`. Both build the fake image from dated
   fixture blocks (`# YYYY-MM-DD <topic> (#n)`, one per catalog revision),
   then make the default `run` with its `check "name" "expr"` lines, then
   the `--no-secrets`, `--full`, `-u` and other runs, each with its checks.
   - The new tool's files go in a fixture block, its default-run checks
     after the existing ones, and a check depending on another run after
     that run's checks, under the same dated label at each place. A
     separate root and `run` is only for a layout the main image cannot
     hold.
   - Add at least one check per new secret glob, exclusion and discovery
     source, and one for anything unusual (symlinks, SQLite sidecars,
     spaces, drive-letter or `file:///` project references), with identical
     check names in both files.
   - A secret glob's check asserts the fixture file's row with
     `secret: true` (and `skipped_secret` under `--no-secrets`). A secret
     glob under an entry that names a directory's children needs its own
     catalog line (as the `.claude-*/` relocated-home globs do).
   - The fixture holds one file per credential name in the research
     document's Credentials section, not one per glob.
   - A check names a row by its full path from the image root, never a
     suffix.
   - Run the full shell matrix of
     [Validation and testing](#validation-and-testing), Windows PowerShell
     5.1 through the WSL path included.
7. Write the Collection part of `research/<agent>.md`, and the Parsing part
   if the tool keeps transcripts worth parsing; add the agent to the index
   table in [`research/README.md`](research/README.md), with its Parser and
   Notes cells; note the tool in
   [`collectors/docs/coverage.md`](collectors/docs/coverage.md) if needed;
   close the matching issue. Add the agent to the
   [agents page](docs/agents.md) table as `backlog` (parseable records, with
   a parser issue) or `collected` (nothing to parse, with the reason),
   linked to a new subsection under
   [Collected agents](docs/agents.md#collected-agents) listing what the
   collector gathers. A `collected` agent also goes into `NO_TRANSCRIPTS`
   in `analyzer/doubleagent/parsers/__init__.py`, which the analyzer
   reports as `collected only`. A parser issue filed
   later is linked in the row by the commit that files it, and examples of
   the agent list (`agents:`, `detected_not_parsed`, `collected_only`)
   change when the agent appears in the fixture.

Source validation gets the lines into the tables; a per-step real-install
round on every platform and surface the agent ships, one listing diff (the
change between listings of the agent's directories before and after a user
action) per user action, completes the addition
([Real-install rounds](#real-install-rounds)). An agent added from source
alone gets its round filed as an issue, and its evidence cell in the index
table names a real install only after the round.

## Validating catalog entries from source

Repeat this process when a tool is added or its layout may have changed,
before any install.

1. **Group the tools and fan out.** One researcher per platform group, in
   parallel. Forks of one codebase share a group (Cline with Roo Code and
   Kilo Code); closed-source tools get their own groups.
2. **Give each researcher the same brief:** the tool's current catalog
   lines (from `--list`), the repository to shallow-clone into the
   scratchpad, the grep terms, and the instruction to cite file and line
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
   a thousand words, plus a "Proposed lines" section in the collector's
   formats (`agent|glob` for `CATALOG`, globs for `EXCLUDES` and
   `SECRET_GLOBS`, `project|glob` for `PROJECT_CATALOG`), dropped when the
   lines land. The build named in an Observed bullet is the version the
   evidence rests on; an install of another build that differs from it
   means a new round.
4. **Evidence order.** Best first; the report names each claim's level,
   and the evidence column of the index table in
   [`research/README.md`](research/README.md) records it.
   1. A real install: a per-step round
      ([Real-install rounds](#real-install-rounds)) on Windows, macOS and
      Linux.
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

   A question about the product's surface (where a menu item lives, how a
   chat is deleted) or its behaviour (when a retention sweep runs) is
   answered first from the vendor's documentation and forum, found by web
   search or the documentation index (`llms.txt`) and cited as that source;
   a VM experiment then tests the documented claim. Grep the bundle only for key names, enum
   values, record shapes and path strings, or to confirm a string a page
   names.

   For minified bundles and compiled binaries:

   - Grep for a stable fragment of a message, never the whole sentence, as
     log lines are usually template literals
     (`${h?"Enabling":"Disabling"} remote control for session ${s}`); "not
     in the bundle" needs a second fragment to miss.
   - A compiled CLI keeps its source strings at offsets that differ per
     platform build: cite `<tool> <version> <platform> @<byte offset>` with
     the literal snippet and record the artifact's sha256 and origin, as
     [`research/claude-code.md`](research/claude-code.md) does for
     `claude 2.1.295 linux-arm64`. The `app.asar` form is
     `app.asar:<path>@<offset>` with the snippet.
5. **Integrate in one commit.**
   - Research agents write only their research documents. One integration
     agent makes every table change after all reports are in, under the
     vendor's comment block (or a new one), never at the end.
   - It splits each group report into one document per catalog agent,
     named exactly as in `--list`, keeping citations and fixtures and
     dropping the proposed lines. Every agent with a catalog line gets its
     own document; [`research/frameworks.md`](research/frameworks.md) keeps
     only the sweep's summary table and the evidence for what got no line.
     Agents sharing a lineage each say so and link the other. Grouped
     reports are never committed.
   - The research README index lists every document, alphabetically by
     agent name, and holds the cross-agent observations.
   - It then makes the table, discovery and smoke-test changes of
     [Adding an agent to the catalog](#adding-an-agent-to-the-catalog),
     runs the full shell matrix and a live collection (reading the
     `skipped_excluded` and `secret: true` rows), updates the research index
     table and bumps `VERSION`.
6. **Close the loop on GitHub.** Close each tool's issue with a comment
   naming the evidence source and commit, the key findings and the catalog
   changes. Comment on the analyzer parser issue with any SQLite-only
   sources. File issues for anything out of scope the research surfaced.

### Research document structure

The research documents are the source of truth for what the project knows
about an agent: the catalog is written from the Collection part, the parser
from the Parsing part, and other pages and docstrings link the section
holding a fact rather than restating it. Every `research/<agent>.md` uses
the outline below, with the same heading names (they are the anchors), and
puts unique subsections under the section owning the fact.

A document covers one catalog agent, and a catalog agent is one state
directory. A surface is its own catalog agent and document when the catalog
can attribute its state directory on its own, at most through a nested
entry (`copilot-cli`, `copilot-desktop` and `copilot` are three). Surfaces
writing one shared tree (the Cursor IDE and its embedded agent CLI) are one
agent and one document, with a `####` subsection per surface under
Per-user storage and under Transcript stores, which says which surface
writes each store, and one Observed bullet per cell (one surface on one
platform). A surface is never split out to shorten a document nor merged in
to save a catalog line.

`tools/check_research_structure.py` enforces the outline, the split phrases
and the placement of confidence paragraphs; with `--before <dir>` holding
the previous copies it lists every citation, code span, fenced line, number
and label a rewrite dropped.

The document opens with `# <agent>` (the catalog name) and one unheaded
intro paragraph: what the agent is, its surfaces, its lineage and the
parser's state in one sentence. Nothing else precedes
`## Source and evidence`. A bold label never stands in for a heading; prose
wraps at 100 characters, except a citation link or a lone code span; tables
stay tables.

1. `## Source and evidence`: the repository and commit (or bundle, binary,
   documentation), the evidence level per claim, then `### Observed` and
   `### Source reviews`.
   - An Observed bullet is one platform and surface: build, date, the steps
     run and what each wrote, in one paragraph; "No real install." when
     none. Its findings are stated in the sections that own them and linked
     from the bullet.
   - What the collector or analyzer did during a round goes to the docs
     page it concerns or to an issue, never into the document.
   - A Source reviews entry is the date, the commit or build reviewed, and
     one or two sentences on what changed in the document, newest first;
     "None." when none. Its facts go into the owning sections, which never
     cite the entry.
2. `## Collection`, the collector's half:
   - `### Per-user storage`: every directory and file under the home per
     platform, what each holds, sizes where observed; `####` subsections
     per platform or product when layouts differ; the fixed
     `#### Scratch directories` ("None known." when none).
   - `### Credentials`: credential files by exact name, keychain use and
     its file fallback, config files that can embed keys, identity values
     to redact.
   - `### Exclusions`: large or low-value subtrees as exact patterns.
   - `### Project-local files`: files written or read inside repositories.
   - `### Where the project path is recorded`: file and key, or table and
     column, that discovery or the analyzer reads.
3. `## Parsing`, the analyzer's half; an agent with nothing to parse has
   one paragraph here (collected only, or the parser issue) and no `###`
   sections:
   - `### Transcript stores`: where transcripts live and how they are
     named, per surface; the fixed `#### What survives a delete`: what the
     agent's own delete, clear, archive or retention removes, what remains
     (the history file, second copies, deletion markers, copies held by
     other agents, project-local traces) and what the analyzer reads, with
     a link to [Secondary sources](analyzer/docs/output.md#secondary-sources)
     and no analyzer version.
   - `### Record schema`: line or row types and their key sets, `####` per
     store or record family; the field names the parser depends on.
   - `### Joins and timestamps`: session id, project path, tool-call ids,
     the time basis and precision, what cannot be joined.
   - `### Other stores`: databases, indexes, histories, logs, mirrors and
     backups beside the transcript store, `####` per store.
   - `### Format versions`: how the shapes changed and which build writes
     which.
   - `### Secrets`: what inside the stores is a credential or an identity.
   - `### Parser plan`: the row table and the globs, then the fixed `####`
     subsections in this order, each present and saying "The agent records
     none." or "Not stated." when that is the fact: `Permission rows`,
     `Model rows`, `Past-use rows and secondary sources`,
     `Sync, relay and origin rows`, `Scratch directory rows`,
     `Deletion rows`, `Bad-line rows`, `Fixtures`; unique subsections
     follow them.
4. `## Confidence`: one list for both halves, confidence per item and what
   could not be determined, holding no fact the body states and no item it
   has answered. A confidence paragraph elsewhere is a defect.

One home per fact: Per-user storage names each file and what it holds;
Transcript stores through Other stores describe its records; Credentials
names credential files and Secrets the stored fields that are credentials;
other sections link the home. A fact stated in two sections is a defect
even when the copies agree. The document speaks as one: it never refers to
"the collector document", "the analyzer review", "the two documents" or a
"collector concern"; a dated subsection ("Found in the review of ...") is
folded into its topic; a section link points at the topic it names.

`research/README.md` is the index (evidence level and parse status per
agent, and the survival table); `research/frameworks.md` (one section per
framework) keeps its own shape.

#### Research rounds with subagents

The split in [CLAUDE.md](CLAUDE.md) applies: the main session writes one
brief and a per-tool assignment, one research subagent per tool runs in
parallel, one integration subagent makes the table, test and index
changes, and the main session reviews. The brief sets the safety envelope:

- Fetch untrusted code; never execute it. Clones, `npm pack` tarballs and
  binaries for `strings` go in the scratchpad and are only read: no
  `npm install`, no installer, no running the tool. A real-install run in
  `lab/` is a separate step the user asks for explicitly.
- A blocked clone or download is reported, never worked around from
  memory.
- A subagent that runs a live collection against the workstation writes it
  under the scratchpad, deletes it afterwards, and reports no file contents
  or manifest rows from it; or the main session runs it.
- The subagent reports what to post to GitHub; the main session posts it.

The review gate is mechanical first: run `tools/check_research_links.py`
(with `--repos` pointing at the scratchpad checkouts when they exist) and
treat any unresolved link, and any claim without a link, as unverified.
Then the main session repeats the integration subagent's test run:
`collectors/tests/catalog-sync.sh`, the full shell matrix and the analyzer
tests.

#### Research checks

- A library with no platform branching (`xdg-basedir`, `etcetera`,
  `home.Config()`) means `~/.config` and `~/.local/share` on macOS and
  Windows too.
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
  collect one directory twice on macOS and Windows: add both only when
  distinct on Linux, and make their smoke check test attribution, not
  count.
- The integration agent derives the number of agents from the documents in
  the tree, not the brief, and says when they differ.
- Claude Desktop since the 2.31226 builds (October 2026) has one Chat and
  Cowork surface, which enters Cowork mode itself, beside the Code tab.
  Never describe a "Cowork mode" the user starts or ask the user to "open
  Cowork", and never expect a `local_<uuid>.json` under
  `local-agent-mode-sessions/` for every chat: on 2.31226.1 (MSIX build and
  Linux deb) a chat that wrote files ran as a remote session leaving only
  `remote-session-spaces.json` and the files it committed, and scheduled
  tasks set from a chat stayed server-side, not in `scheduled-tasks.json`.
- Chat history outside the home cannot be a catalog line: record it in
  [`collectors/docs/coverage.md`](collectors/docs/coverage.md) as a manual
  step and file an issue. Docker and Podman named volumes are the exception
  since collector 1.5.0: they go in `DOCKER_VOLUMES`.

## Real-install rounds

A round installs one agent in a clean VM (or the `lab/` sandbox for a
Linux-only CLI), never on the workstation, exercises it, and compares what
it wrote with the research documents, the catalog and the parser.
[`docs/lab-vms.md`](docs/lab-vms.md) describes the VMs.

A round covers every surface the agent ships on every platform it ships it
for; each cell is its own round, with its own Observed bullet. A round is
required, before any table, document or parser changes, when an agent is
added to the catalog, when its coverage is extended (a cell without an
Observed bullet, a feature the documents do not mention), and when a
version newer than the last Observed bullet's is in use; an install of
another build is a new cell, never a confirmation of the old one.

1. **Survey names first, one step at a time.**
   - For a CLI agent the main session performs each step: it runs the
     agent in a detached `screen` session in the VM over SSH, sends
     keystrokes with `screen -S <name> -p 0 -X stuff`, feeds a paste
     through a register file carrying the bracketed-paste markers (`stuff`
     truncates long strings), and reads the terminal with `hardcopy`. The
     person does only what needs a browser, a phone or the VM's GUI, and
     says when it is done. A browser step is the person's step, never a
     stop point: it is completed, with a throwaway account where one
     exists, not cancelled. A GUI agent is the person's to drive.
   - After each step the main session takes a listing of the agent's
     directories (names, sizes and mtimes only, by a script in the VM's lab
     directory) and diffs it against the previous one before the next step;
     the diffs are the Observed bullet's evidence. A round that ran every
     task before the first listing says so in its document and gives the
     modification times.
   - Listing names sort in the order taken: zero-padded step numbers, an
     unplanned event taking the next number, never a letter.
   - The survey's roots are fixed before the baseline and include every
     surface the round will touch; a root added mid-round names its first
     listing as its baseline. The survey also counts the agent's processes
     by name.
2. **Read shapes, never values.** `analyzer/tools/shape.py` prints key
   names, value types, JSONL line types, byte totals and the enum tokens of
   a fixed set of paths, every value through its redaction function, so a
   UUID, a hex id, a path or a URL prints as its class; a new printing path
   gets a test feeding it those four forms. Copy it to the VM alone and run
   it with `python3 -I`; write no ad-hoc script that prints values. Nothing
   from a real install goes into the repository, an issue or a report
   beyond file names, key names, types, sizes, enum values and log lines
   with ids redacted.
3. **Exercise every surface separately.** Each item is its own step with
   its own listing diff, run for every kind of task the surface offers.
   - Before the baseline, check the brief's step table against this list and
     the previous cell's lessons, and fix it. Before the first prompt,
     record the REPL's `?` shortcut list and `/help`.
   - A question without tools, a session with a tool call, a task that
     writes files, a scheduled or automated task, a plugin or MCP server, a
     resumed session, and a short and a long paste as prompts, each paste
     its own step.
   - One tool call approved, one denied, an "allow always" or equivalent
     grant, and a model switch mid-session. Record the default permission
     mode at the first tool call; if it auto-approves, switching to the
     asking mode is its own step, with the mode names in cycle order.
   - At every dialog, move through each option and read the screen after
     each move before confirming; option wording and footer hints
     (`Tab to amend`, `Esc to cancel`) are findings and the way to the
     extra options (a reason field, a directory-scoped grant).
   - A session synced to the vendor's account where the agent offers it (a
     resume, a sign-in or a setting may do it unasked), with a listing diff
     showing which files mark it.
   - A source-host integration (GitHub, GitLab, Bitbucket) where offered,
     with a throwaway account: connect through the agent's own integration
     and, in an editor, the editor's provider too; run a remote agent with a
     read-only task ("describe this repository, change nothing, open no
     pull request"); reopen that conversation after a relaunch. Record
     whether the account is named on disk, where its token or grant lives
     (a file, the editor's secret storage, the OS keychain, or server-side
     only), whether the repository list is cached, and whether
     repository-bound conversations are stored, cached or fetched.
   - A source-host CLI the agent leans on (`gh`) is logged in first, as its
     own step, with a listing showing where its token landed (its
     plaintext `hosts.yml` over SSH, the OS keychain from the GUI), then
     the agent's own flow is completed. Log `gh` in with
     `gh auth login --expiring` (gh 2.103.0 or later), only for steps that
     act on the public lab repository and the one-time store observation
     below, and run `gh auth logout` before the VM is released; the logout
     removes only the local token, and the grant itself is revoked under
     the account's authorized OAuth apps.
   - An agent driven over SSH on macOS cannot reach the login keychain and
     writes its credential fallback (Claude Code 2.1.310:
     `~/.claude/.credentials.json`, mode 0600). Record it, then sign out and
     have the person sign in from a Terminal in the VM's GUI, with a listing
     before and after; the same holds for any tool preferring an OS
     credential store. Once per tool and platform; later cells cite the
     research document.
   - A session deleted the way a user would (the agent's own delete or
     clear, or the project directory removed). When the delete offers to
     remove the synced copy, take it and record whether a local file still
     marks the sync (Copilot CLI 1.0.98 marks one with `remote_steerable`,
     `mc_task_id`, `mc_session_id` and `mc_last_event_id` in
     `workspace.yaml`).
   - A set idle period with a listing, then a reboot and a listing after
     login.
   - A surface that writes nothing locally is a finding, and so is a
     feature the VM cannot run; say which. A feature this account or build
     lacks is recorded as not exercised, with the reason.
   - The brief carries the exact prompt per step; a step whose prompt
     misses is kept as a finding about the agent, and repeated with a
     sharper prompt under the next name. An install command is copied from the vendor's page or script,
     never retyped, and pipes to the shell its shebang names. The person
     reports what the screen showed at each step.
4. **Run the collector in the VM** as the user, before and after the
   delete steps; pull the manifests, summary, log and archive. Read the
   error rows, the `skipped_excluded` rows and the unflagged-key list of
   [Validation and testing](#validation-and-testing), diff the two
   manifests, and check that the analyzer gives every observed session rows
   and no `system` row reports an unreadable file. A parser update starts
   by diffing the shape tool's line types and key sets against the research
   document's. The brief's step table pairs each step with the claim it
   tests; the evidence row records settled, contradicted or new. A
   shape-tool change lands with its tests before the next listing.
5. **Write it down the same day.**
   - The Observed subsection gets a bullet per cell: version, build,
     date, what was exercised and what each surface wrote. A layout the
     parser handles that no real install has shown is marked "not observed
     on any real install, bundle-derived".
   - Every statement about a product surface (a tab, a button, a mode)
     carries the build and date; every claim about what an agent writes,
     sanitises or removes names the build or commit it was verified at.
   - What the delete left and removed goes into
     `#### What survives a delete` and `#### Deletion rows`, linked from the
     Observed bullet. Collector and analyzer findings go to an issue or the
     docs page they concern, never the research document.
   - Catalog and parser changes follow the usual checklists, under the
     pending version.
6. **Clean up, after the main session says so.**
   - The main session says, in plain words and as its own line, when the
     VM may be reverted or the `lab/` container brought down: only after
     the last listing, with the collection and probe output on the analyst
     machine and its outputs in the VM deleted. Until then it says the VM
     is still needed. Then delete the pulled collections and revert to the
     clean snapshot before the next agent.
   - Automatic updates and reboots are off in the clean snapshot. The
     person snapshots the VM at every pause with a session open and before
     a hard-to-undo step (a delete, a retention sweep, a reboot).
   - A revert to the snapshot after step N discards every later listing;
     those steps are retaken under new names, and the evidence table says
     which snapshot and why. Mid-round snapshots are deleted when the VM is
     released; the clean snapshot is kept.

Subagents integrate from one brief file holding every number the documents
will cite, the install facts, the collector and analyzer runs, and the
assignments. They never run the collector against the VM or read its
user's data, and the main session posts to GitHub. Two run in parallel on
disjoint files: one for the collector side (tables, discovery, smoke
fixtures, collector docs, changelog), one for the research document and
the agents page; each reports every check it ran with the result line.

The main session then re-runs the checks (catalog drift, analyzer suite,
markdownlint, link check, the sh smoke test under `/bin/sh` and the
PowerShell smoke test under Windows PowerShell 5.1 on the VM), runs the
merged parser over the round's archive, compares each new row form's count
and the fixtures' values with the real records, reviews the diffs against
the brief, and commits. When the research and parser subagents run in
parallel, the Parser plan's row forms are copied from the parser's
docstring at review.

## Validation and testing

CI (`.github/workflows/ci.yml`) runs the catalog drift test; shellcheck;
the sh smoke test under sh, dash, bash, busybox ash, zsh and posh; the
drift test and sh smoke test under dash on macOS (BSD tools); the
PowerShell smoke test under PowerShell 7 on Linux and Windows PowerShell
5.1 on Windows; the analyzer tests on Python 3.10 to 3.14; the lint job of
[Quality gates](#quality-gates); and a `CI result` job that fails when any
of them failed or was cancelled. `CI result` is the one status check the
ruleset on `main` requires.

A change touching only Markdown files and `.markdownlint-cli2.jsonc` is
documentation only: CI skips every other job and the lint job runs only
markdownlint, `tools/check_doc_links.py` and
`tools/check_research_structure.py`, which are also all such a change needs
locally. A push whose previous commit GitHub cannot name (a new branch, a
force push) counts as a code change. Before committing a collector change,
run:

```sh
cd collectors
shellcheck -s sh -S warning collect-agent-artifacts.sh   # info-level notices are allowed
tests/smoke.sh            # /bin/sh
tests/smoke.sh dash
tests/smoke.sh bash
printf '#!/bin/sh\nexec busybox ash "$@"\n' > /tmp/ash && chmod +x /tmp/ash && tests/smoke.sh /tmp/ash  # busybox, as CI runs it
printf '#!/bin/sh\nexec zsh --emulate sh "$@"\n' > /tmp/zsh-sh && chmod +x /tmp/zsh-sh && tests/smoke.sh /tmp/zsh-sh
tests/smoke.sh posh
tests/catalog-sync.sh
pwsh -NoProfile -File tests/smoke.ps1
cd ../analyzer && tests/run.sh
cd ..
# quality gates, from the repository root (see "Quality gates")
uvx ruff@0.16.10 check . && uvx ruff@0.16.10 format --check .
uvx --from pyright==1.1.414 --with zstandard --with python-dateutil pyright
npx -y markdownlint-cli2@0.23.3 "**/*.md"
python3 tools/check_doc_links.py
python3 tools/check_research_structure.py
shfmt -ln posix $(shfmt -f .) >/dev/null && shellcheck -s sh $(shfmt -f .) && checkbashisms -p $(shfmt -f .)
pwsh -NoProfile -Command '$r = @("collectors/Collect-AgentArtifacts.ps1","collectors/tests/smoke.ps1") | ForEach-Object { Invoke-ScriptAnalyzer -Path $_ -Settings collectors/PSScriptAnalyzerSettings.psd1 }; $r | Format-Table -AutoSize; exit @($r).Count'
```

For a change to the PowerShell collector, also run the test under real
Windows PowerShell 5.1. From this WSL checkout that is:

```sh
/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -ExecutionPolicy Bypass \
  -File '\\wsl.localhost\Ubuntu\home\sean\dev\coding-agent-collector\collectors\tests\smoke.ps1' \
  -Collector '\\wsl.localhost\Ubuntu\home\sean\dev\coding-agent-collector\collectors\Collect-AgentArtifacts.ps1'
```

Without the privilege to create symlinks the test skips the symlink checks
and says so. A pass under PowerShell 7 alone is not enough, and a pass
under 5.1 through the WSL path is weaker than one on the CI runner. A change
is done when every check on its pull request, CI and CodeQL, is green;
watch them with `gh pr checks --watch` and fix what fails before reporting
the pull request ready.

The smoke test runs the collector in image mode over a fake disk image
under `$TMPDIR`, with the given shell for the script and the `find -exec`
workers (`COLLECTOR_SH`). Every check passes under every shell; a failure
under one shell is a portability bug.

On Debian and Ubuntu `/usr/bin/ash` is dash, so test busybox with the
wrapper above. CI installs `busybox-static`, whose `ash` runs busybox's own
applets before anything on `PATH`, so a test that forces a code path with a
`PATH` shim needs the static build: `apt-get download busybox-static &&
dpkg-deb -x busybox-static_*.deb bbs` in the scratchpad, then point the
wrapper at `bbs/usr/bin/busybox`.

Then run a live collection against your own host and read the summary:

```sh
collectors/collect-agent-artifacts.sh -o /tmp/live -q
powershell.exe -ExecutionPolicy Bypass -File collectors\Collect-AgentArtifacts.ps1 -OutputDir $env:TEMP\live -Quiet
```

Check the error count, the `skipped_excluded` rows for anything new and
large, and that the manifest parses as JSONL. List the `collected` rows not
`secret: true` whose file name contains `key`, `token`, `secret`, `cred`,
`cookie` or `auth`, and account for each: not a credential (say why in the
research document) or a new secret glob. Delete the output afterwards.

The test uses `python3` only for JSON validation and the hash cross-check,
skipped without it. Never add a `python3` requirement to the collector.

### Waiting for long-running checks

- Wait on the process you started: `wait` in the same shell, or each job
  writes its exit code to a file (`...; echo "exit $?" >"$out.rc"`) and the
  wait is for every `.rc` file. Never poll with `pgrep -f` or `ps | grep`,
  which match the waiting shell itself.
- Bound every wait (`timeout 1800 ...`, or a loop of N rounds); a timeout
  is a failure.
- Read results from the output files, not from whether the watcher ended.
- For CI, use `gh pr checks <n> --watch` or
  `gh run watch <id> --exit-status`.

### Quality gates

Every gate runs in the CI `lint` job, never in the pre-commit hook. The
versions are pinned in the workflow's `env` block; change it and this table
together. Run the commands from the repository root before pushing; a
documentation-only change needs only the last two rows and markdownlint.

| Gate | Version | Local command | Configuration |
| --- | --- | --- | --- |
| ruff lint | 0.16.10 | `uvx ruff@0.16.10 check .` | `ruff.toml`; `analyzer/pyproject.toml` extends it |
| ruff format | 0.16.10 | `uvx ruff@0.16.10 format --check .` | `ruff.toml` (line length 100) |
| pyright, standard mode, Python 3.10 | 1.1.414 | `uvx --from pyright==1.1.414 --with zstandard --with python-dateutil pyright` | `pyrightconfig.json` |
| markdownlint | markdownlint-cli2 0.23.3 | `npx -y markdownlint-cli2@0.23.3 "**/*.md"` | `.markdownlint-cli2.jsonc` |
| shfmt POSIX parse | 3.14.1 | `shfmt -ln posix $(shfmt -f .) >/dev/null` | none; never pass `-w` |
| shellcheck | 0.11.0 | `shellcheck -s sh $(shfmt -f .)` | `.shellcheckrc` |
| checkbashisms | devscripts v2.26.13 | `checkbashisms -p $(shfmt -f .)` | none |
| PSScriptAnalyzer | 1.25.0 | the `pwsh` line in the block above | `collectors/PSScriptAnalyzerSettings.psd1` |
| document links | none | `python3 tools/check_doc_links.py` | none; see the link rule under "Conventions" |
| research document structure | none | `python3 tools/check_research_structure.py` | none; see [Research document structure](#research-document-structure) |

- `shfmt -f .` finds sh files by shebang as well as extension, so the hook
  script, `lab/lab.sh` and `analyzer/tests/run.sh` are checked too.
- pyright runs with both analyzer dependencies installed; `zstandard` and
  `python-dateutil` are ordinary top-level imports, never lazy imports with
  a fallback.
- shellcheck, shfmt and checkbashisms are not on PyPI or npm: install the
  release binaries (CI checks their sha256), and checkbashisms as the
  single `scripts/checkbashisms.pl` from the devscripts tag or
  `apt install devscripts`. PSScriptAnalyzer installs with
  `pwsh -NoProfile -Command 'Install-Module PSScriptAnalyzer -Scope CurrentUser -RequiredVersion 1.25.0 -Force'`;
  CI instead downloads the same nupkg from the
  [PSScriptAnalyzer release](https://github.com/PowerShell/PSScriptAnalyzer/releases)
  on GitHub, checks its sha256 and unzips it into the user module path,
  because the Gallery's v2 API fails intermittently and `Install-Module`
  does not retry.
- The one `ruff format` commit is listed in `.git-blame-ignore-revs`;
  enable it with `git config blame.ignoreRevsFile .git-blame-ignore-revs`.
- The pre-commit hook (`.githooks/pre-commit`, enabled once per clone with
  `git config core.hooksPath .githooks`) holds only fast operations, today
  the catalog drift check; linters, tests and network queries belong in CI
  or the release checklist.

## Conventions

- The project's name is DoubleAgent in prose, titles, the site name and
  descriptions; `doubleagent` is the command, package, repository, every
  path and URL, and stays lowercase in code spans.
- A commit message has an imperative subject under 70 characters and a
  body explaining why.
- Everything an AI agent writes to git or GitHub (commits, issues, pull
  requests, comments, reviews, review replies), including on a person's
  behalf, discloses AI generation and names the model as the tool reports
  it, both models when a subagent wrote and the main session reviewed. A
  commit ends with `Co-Authored-By: <model name> <noreply address>`
  (`Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`); anything
  else with a line such as `Generated by AI (Claude Opus 5.5).`
- Every change reaches `main` through a pull request. A ruleset
  (`gh api repos/seanthegeek/doubleagent/rulesets`) forbids direct pushes,
  force pushes and deletion of `main`, and a merge needs the `CI result`
  job and the CodeQL code-scanning analysis green.
- Work on a branch named for the change; open the pull request with
  `gh pr create` (naming the issue, ending with the AI disclosure line),
  watch `gh pr checks --watch`, and report it ready. The user merges it
  unless the task's instructions said to merge. An agent never pushes to
  `main`, never merges a pull request unasked, and never asks for a ruleset
  bypass.
- Reproduce a review finding before fixing or declining it. A finding the
  change introduced is fixed in the pull request with a test; a defect in
  untouched code is filed as an issue, and the reply names the fix commit
  or the issue. A third finding on one function in three reviews means
  rewriting it from its specification.

- Track outstanding work as GitHub issues; file one for anything out of
  scope rather than widening a change. Label `bug` behaviour that is wrong,
  broken or contradicts the documentation, and `enhancement` a new feature,
  catalog entry or improvement to documented behaviour.
- An issue filed with `gh issue create` states the part, version and
  platform as the forms under `.github/ISSUE_TEMPLATE/` ask. A
  vulnerability is never an issue: it goes through the private advisory
  form named in [`SECURITY.md`](SECURITY.md).
- Every bug fixed and feature added has an issue, found among open and
  closed issues or filed before the work is committed. Changes with no
  behaviour of their own (typos, formatting, a test-only refactor) need
  none.
- A commit implementing an issue says `Refs #<n>` in its body, never
  `Fixes` or `Closes`, and each changelog line for it ends with `([#<n>])`.
  GitHub closes an issue for any closing keyword (`close`, `fix`, `resolve`
  and their `-s` and `-d` forms) followed by `#<n>` anywhere in a commit
  message, so write `the fix for #29`, never `fixes #29`. The issue is
  closed after the release is tagged, with a comment that explains the fix
  and names the commit and release.
- Evidence lives in the research documents, not in issues: the commit that
  files or acts on an issue recording file names, keys, field values or
  offsets puts those facts, cited, in `research/<agent>.md`; the issue
  keeps the proposal and a pointer.

- Documentation describes the code, not the intent. Check every sentence
  stating an option, an output file, a field, a value or a behaviour
  against the code before committing, and say where the collectors or
  platforms differ and what happens without root or Administrator. Never
  write a sentence the code cannot verify.
- Document a feature with specifics (an option table row, a file-by-file or
  field-by-field table, the list of values), never in passing. A reader
  with only the README must be able to say what each output file holds and
  which command produced it.
- Run a command block the reader will paste as written, in a fresh shell on
  its platform, and write into it what a fresh shell lacks (a PATH entry
  an installer prints but does not apply, an executable the same session
  installed).
- Before a `VERSION` bump, re-derive the option, output file and status or
  column lists from the code and diff them against the README; the
  changelog entry records what changed.
- Example output blocks and key, field and column tables are part of the
  interface. A change to anything a command prints or writes (a stdout or
  log line, a manifest field or status, a `collection.json` key, the
  `--list` output, an option, a JSON or JSONL field, a CSV column, a row
  text convention) updates every example and table showing it, in the
  READMEs and `docs/` pages of both parts, in the same commit; adding a
  parser or catalog agent also updates every example showing it under
  `detected but not parsed`, `collected only` or `parser: false`, and the
  [agents page](docs/agents.md) table gains a row per new agent and a link
  per parser issue filed or closed, in the commit that adds the agent or
  acts on the issue. Grep the documents for the name, key,
  label or heading. An example marked as a real run is regenerated by
  running the command (the collector over the smoke test's fake image, the
  analyzer over the test fixtures), never edited by hand. A behaviour
  change without a README update is incomplete.
- The [top-level README](README.md) and [`docs/index.md`](docs/index.md),
  the site's home page listing each part's pages, are overviews only. A new
  page joins that list and the `nav:` tree in `mkdocs/mkdocs.yml` beside
  its siblings; a page belonging to neither part, such as
  [`docs/agents.md`](docs/agents.md), goes under `docs/` at the root and at
  the tree's top level. `mkdocs build --strict` fails on a document missing
  from the tree. Documents carry no front matter; a page's title is its
  `# Title`. The [documentation site](https://seanthegeek.net/doubleagent/)
  gives every heading the anchor `tools/check_doc_links.py` accepts.
- Documentation addresses its reader, never the agent maintaining it: the
  READMEs and pages under `collectors/docs/`, `analyzer/docs/` and `docs/`
  the responder or analyst; `testing.md`, `development.md`, `internals.md`,
  `lab-vms.md`, the lab README and the research index a contributor. Only
  this file holds the rules for making a change.
- This file states rules, not history. A new rule is one or two enforceable
  sentences in the section that owns it, written as the lesson taught, not
  the incident that taught it: no narrative, no date, and no issue or pull
  request link as its provenance (a live pointer the reader must open, such
  as the issue forms, the ruleset or [`SECURITY.md`](SECURITY.md), is the
  exception). A concrete value (a glob, a name, a row form) stays only when
  the rule depends on it; an example stays only when it is the
  specification; a rule already stated elsewhere in this file is linked, not
  repeated. The same holds for `CLAUDE.md`, and `tools/check_doc_links.py`
  fails on an issue or pull request link in either file.
- An index table (the agents page, the survival table) gives a short cell
  and a link, never the documents' detail.
- A document never says how it is maintained, never states its own
  structure as a rule, and never cites an issue to justify a sentence. An
  issue link in a user page marks only open work or a behaviour that
  changed at a version.
- A heading is followed by content, or by a paragraph that says what the
  content means for the reader, never by a description of what the section
  lists. A page's opening paragraph says what the thing is and when the
  reader uses it; it never lists the page's topics or tags its audience.
- Prose names items or says "some", never a count. Maintainer prose in a
  document is a sign to check the facts around it.
- A page about an environment states what the host must provide and why,
  never one machine's chip, memory or disks. A lab snapshot holds only what
  a developer's machine would have, never this repository's CI gates.
- A term a document takes from a vendor, a bundle or a product surface (an
  abbreviation such as MSIX, an internal name such as `ccd`, a directory or
  key whose name does not say what it holds) is explained at its first use
  in that document: what it stands for, what it denotes and where the name
  comes from; a research document glosses a term that runs through it in
  its intro. Before committing, check that each capitalised or letter-digit
  short token is a code identifier the sentence explains or a term glossed
  earlier.
- This file's terms of art (cell, round, surface, Observed bullet, listing
  diff, secondary source, nested entry) are ordinary English elsewhere: a
  page introduces one before relying on it (a heading holding it, a first
  use linking its definition, a parenthesised gloss) or says the thing in
  plain words. `tools/check_doc_links.py` enforces this on every page but
  this one, `CLAUDE.md` and the research documents.

- A reference from one project document to another is a Markdown link,
  never a bare path or code span: a file as
  ``[`collectors/docs/coverage.md`](collectors/docs/coverage.md)``,
  relative to the citing file; a section as
  ``[Transcript stores in `research/claude-desktop.md`](../research/claude-desktop.md#transcript-stores)``,
  with the fragment GitHub derives (lower case, punctuation other than
  hyphens removed, spaces to hyphens); one link per section. The exceptions
  are this file's references by name to the sections of
  [Research document structure](#research-document-structure), a template
  path with a placeholder (`research/<agent>.md`), and a catalog path
  ending in `.md` (an agent's `AGENTS.md` inside a project).
- An external source is one link whose text names it (a page title without
  quotes, `owner/repo issue 105`) and whose target is the full URL with its
  scheme, never a bare `claude.com/docs/...` or a URL as link text. A
  fetch date or note goes in parentheses after the link; a second page on
  the same site is a second full link. Citations to a file at a commit keep
  their form
  ([Validating catalog entries from source](#validating-catalog-entries-from-source)).
- A commit hash in prose, a table cell or a code span is a link,
  `[<short hash>](https://github.com/<owner>/<repo>/commit/<full sha>)`
  (a code span kept inside the link text), with the repository and sha
  from the agent's research document citations; a closed-source binary's
  build hash stays as written.
- An issue or pull request is one link each, such as
  `[issue 92](https://github.com/seanthegeek/doubleagent/issues/92)` or
  `[pull request 129](https://github.com/seanthegeek/doubleagent/pull/129)`;
  a bare `issue <n>`, `PR <n>` or `#<n>` in prose or a table is a defect,
  except the changelogs' `([#<n>])` and a number inside a code span.
- `tools/check_doc_links.py`, in the CI lint job, enforces these rules and
  fails on a `section <n>`; run it before pushing a documentation change.

- Bump `VERSION` in the script for any change to output format or options;
  for the analyzer, in `doubleagent/__init__.py`, which `pyproject.toml`
  reads. Bump once per release: while `VERSION` is ahead of the last
  release tag, later work goes into that version's changelog entry, dated
  at tag time, and `VERSION` is not bumped again unless the user says so.
  A documentation-only change bumps nothing and needs no changelog line.
- Every `VERSION` bump has an entry in
  [`collectors/CHANGELOG.md`](collectors/CHANGELOG.md) or
  [`analyzer/CHANGELOG.md`](analyzer/CHANGELOG.md) in Keep a Changelog
  form: a `## [x.y.z] - YYYY-MM-DD` heading with `Added`, `Changed`,
  `Fixed` and `Removed` subsections, one line per item, written for that
  part's user, calling out any change to the manifest, JSONL or CSV
  interfaces. Work between bumps goes under `## [Unreleased]` and moves
  into the next version's heading at the bump.
- A changelog issue number is a reference link, `([#<n>])`, never a bare
  `(#<n>)`. Its definition,
  `[#<n>]: https://github.com/seanthegeek/doubleagent/issues/<n>`, is
  added with the first line citing the issue, at the bottom after the
  version comparison links, in descending order. markdownlint MD052 (with
  `shortcut_syntax` on) fails on a reference without a definition, MD053 on
  a definition nothing cites.
- A release requires zero open Dependabot alerts, which CI cannot check.
  Before tagging, run
  `gh api "repos/$(gh repo view --json nameWithOwner -q .nameWithOwner)/dependabot/alerts?state=open" -q length`
  and resolve or dismiss, with a reason, each alert it counts.
- Tagging is the user's decision. An agent never creates or pushes a tag
  unless the user asked for that tag in the current session; a bumped
  `VERSION`, a finished changelog or green CI is not that permission. Once
  the bump commit has reached `main` and the user has said to tag it, tag it
  `collector-vX.Y.Z` or `analyzer-vX.Y.Z` with an annotated tag, never a
  bare `vX.Y.Z`, push the tag, and add the version's comparison link to the
  changelog.
- A tag marks a change to what the collector or analyzer does. A change
  touching only documentation, research documents, this file or the linter
  configuration gets no tag and no `VERSION` bump; a pending tag may sit on
  it or on the last code commit, as the user prefers.

## Things to avoid

- Adding Python, jq or any interpreter to the on-host collector.
- Parsing transcripts on the host.
- Collecting model weights, editor caches or extension binaries by default.
- Reading files through symlinks or descending into symlinked directories.
- Writing to `/tmp` by default or anywhere other than `-o`.
- Interactive prompts, `sudo` calls, or anything that reads stdin.
- Bash-isms that happen to work on the developer's machine; test under dash
  and busybox ash.
