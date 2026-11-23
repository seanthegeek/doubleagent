# AGENTS.md

Guidance for AI coding agents and humans working in this repository: the
rules every change keeps. Each area's own rules are listed under
[Where the rules are](#where-the-rules-are).

## Purpose

DoubleAgent is a forensic tool for incident responders: one Go program,
`doubleagent`, built from `cmd/doubleagent` and the packages under
`internal/`. `doubleagent collect` archives the on-disk files of AI agents
for every user on a host or a mounted disk image, with a hashed manifest;
`detect`, `timeline` and `fleet` read a collection, or any directory tree,
and turn the agents' transcripts into a JSONL timeline and summaries; and
`catalog` prints the catalog. The catalog takes any agent, editor or not,
that executes tools or shell commands on the host and leaves transcripts,
configuration or credentials on disk. When a new kind of agent is added,
widen the wording in the [top-level README](README.md) in the same commit.

The project supplements the responder's EDR and never duplicates it:
process lists, logged-on users, network connections, services, scheduled
tasks, shell histories and any other feature an EDR console provides are
out of scope.

`collect` runs on the host and must work through an EDR remote shell,
locally by a responder, and against a mounted disk image on an analyst
workstation, including a Windows image mounted on Linux or macOS. The
analysis commands run on the analyst's workstation; parsing never happens
on the host. The [agents page](docs/supported-agents.md) lists every catalog agent
with its parse status.

## Where the rules are

An area's `AGENTS.md` holds the rules for work inside it, and its sibling
`CLAUDE.md` imports it for Claude Code. Read the one for each area a change
touches before changing it:

| File | Holds | Read it when |
| --- | --- | --- |
| [`internal/AGENTS.md`](internal/AGENTS.md) | the Go standards | changing any Go file under `cmd/` or `internal/` |
| [`internal/catalog/AGENTS.md`](internal/catalog/AGENTS.md) | the catalog's rules and the checklist for adding an agent to it | changing `internal/catalog/catalog.txt`, the fixture image or a research document's Collection part |
| [`internal/parsers/AGENTS.md`](internal/parsers/AGENTS.md) | the analysis standards and the checklist for adding a parser | changing the parser kit, a parser under `internal/parsers`, the registry, or the analysis output |
| [`internal/parserkit/AGENTS.md`](internal/parserkit/AGENTS.md) | a pointer to the parser rules, which apply to the parser kit | changing the parser kit under `internal/parserkit` |
| [`internal/parserregistry/AGENTS.md`](internal/parserregistry/AGENTS.md) | a pointer to the parser rules, which apply to the registry and its growth tests | changing the registry or a growth test under `internal/parserregistry` |
| [`research/AGENTS.md`](research/AGENTS.md) | validating catalog entries from source and the research document outline | writing or changing a research document, or researching an agent |
| [`lab/AGENTS.md`](lab/AGENTS.md) | real-install rounds | running or writing up a round in a lab VM, or changing the lab pipeline's step tables |

An area file is read only by an agent that works inside the area or is
told to read it: a brief for a subagent names every area file its work
needs, beside this file.

## Repository layout

[Repository layout in `docs/development.md`](docs/development.md#repository-layout)
maps the tree and says how to run each tool. Paths in the `AGENTS.md` files
are relative to the repository root unless stated otherwise.

## Standards every change keeps

**Forensic soundness.** `collect` never modifies, moves or deletes
anything outside its output directory, and the analysis commands write
only under `-o` and the directory they extract an archive into. Every file
is looked at (`lstat`) before it is copied, so the manifest holds the
times from before the copy; the staged copy is hashed, not the source.
Nothing follows a symlink: each is recorded with its target and recreated
in the archive. Everything skipped (an exclusion, an oversized file, a
secret under `--no-secrets`, a special file) is recorded in the manifest
with a status and a size. Credential files are collected by default and
flagged `secret: true`. No secret and no file content is printed to stdout
or the log. [`internal/AGENTS.md`](internal/AGENTS.md#go-standards) says
how the Go code keeps each.

**Nothing needed on the host.** `collect` is one static binary with no
dependency: no interpreter, no library, no installed tool. It runs only
`getent`, `dscl` and `plutil`, for the user list and launchd values no file
gives ([Programs run](internal/AGENTS.md#files-programs-and-paths)).

**Non-interactive.** No prompt, nothing read from stdin, every output under
`-o`, concise stdout, and exit `0` when an archive was written even if some
files failed.

**The log is the responder's timing view.** Every log line carries a
timestamp, and a line about one file follows the match line it belongs to,
so the log shows where a run spent its time, match by match; no change
batches matches in a way that moves a `copy failed:` line away from its
file.

**The catalog is data.** Agent paths live in the seven tables of
`internal/catalog/catalog.txt`, never in code; a new agent is new lines.
Every catalog entry is tried against every home directory, and every
`STARTUP_FILES` line against each base of its scope. The table rules are
in [`internal/catalog/AGENTS.md`](internal/catalog/AGENTS.md).

**The outputs are interfaces.**

- The fields, status values (`collected`, `symlink`, `skipped_excluded`,
  `skipped_size`, `skipped_secret`, `skipped_special`, `error_copy`,
  `skipped_unmatched_volume`, `error_read`) and `fs/<original path>`
  archive layout of `manifest.jsonl` and `collection.json` are documented
  in [`docs/collection-output.md`](docs/collection-output.md).
- The `timeline.jsonl`, `sessions.jsonl` and `inventory.json` fields,
  their order and JSON types are documented in
  [`docs/timeline-output.md`](docs/timeline-output.md), and the
  `fleet-inventory.csv` columns in [`docs/fleet.md`](docs/fleet.md).
- Fields and columns are added only at the end. Renaming, retyping or
  removing one needs those pages updated and the commit message saying so.
  Every JSONL record has every field, `""` rather than `null` for no value.
  Timestamps are UTC ISO 8601 with milliseconds and a trailing `Z`.
- The expected-output tests ([Testing](docs/testing.md)) hold the program
  to these pages: a change that alters an output rewrites the expected
  files in the same commit, and the diff is read before it is committed.

## Code style

These rules hold for every Go, Python, sh and PowerShell file, tests
included. `tools/check_code_style.py` enforces the names, the issue and
pull request numbers, the dates and the comment and docstring lengths, with
the Go exemptions of [Go standards](internal/AGENTS.md#go-standards) built
in; ruff's pydocstyle rules, with the `google` convention, check the form of
the Python docstrings ([Quality gates](#quality-gates)).

- **Comments say what the code cannot.** A comment states why the code is
  as it is, the quirk of a platform or format it works around, or the
  specification it follows, in plain words. It never restates the lines
  below it, narrates how the code came to be, or walks through what a
  function does step by step. A fact about an agent's files belongs in
  `research/<agent>.md`, a fact about the program's design in
  [`docs/internals.md`](docs/internals.md); the comment names the page only
  when the reader must open it.
- **No history in code.** A comment or docstring names no issue, pull
  request, date, round, or anecdote of a build ("observed on the 2.1.310
  macOS round"); the research documents and the changelog hold those. A
  commit hash a comment cites as the source of a record shape stays. A
  build or version number stays only where the code branches on it or
  names the version that writes a shape. The copyright line is the
  exception.
- **Short comments.** A run of whole-line comments is at most ten lines,
  the file header excepted. A docstring or a function's comment is at most
  ten lines; a module docstring and a Go package comment, which say what the
  module or package is for, at most thirty. What needs more is a paragraph
  in the owning documentation page.
- **One comment per function**, directly above it: its purpose, its inputs,
  what it returns, prints or writes, and at most one quirk it exists for,
  never a walk through the body's branches.
- **Plain language.** Comments, docstrings, commit messages and
  documentation describe what the code does in words a newcomer
  understands. A domain term is used only where the code implements that
  concept, with a gloss at its first use in the file.
- **Descriptive names.** A variable, parameter, function, type or class is
  named by what it holds or does, as a word or words (`row`, `timestamp`,
  `exitCode`, `handle`, `match`), never by an initial or a home-made
  abbreviation (`msg`, `rec`, `cfg`, `srcPath`); `ABBREVIATIONS` in the
  checker names the common ones and rejects them as a name or a segment of
  one. A name is at least three characters; `_`, `i`, `j` and `k` are
  allowed for a throwaway value or a loop index whose scope is a few lines.
  Abbreviations the project's interfaces spell that way (`mtime`,
  `sha256`, `cwd`, `utc`) and format and protocol names (`json`, `jsonl`,
  `sqlite`, `uuid`, `url`, `id` inside a longer name such as `sessionID`)
  are words. A name an interface fixes (the `TZ` environment variable, a
  pytest fixture such as `tmp_path`, a library's keyword parameter) is
  exempt, and so is a Python class-body field declaration (`at: str`),
  which mirrors an interface key.
- **sh and PowerShell.** The sh scripts (under `lab/` and `.github/`) are
  POSIX sh and pass shfmt's POSIX parse, shellcheck and checkbashisms; in
  sh the name rule applies to the name after a function's temporaries
  prefix (`_ci_` in `_ci_destination`). The PowerShell scripts under `lab/`
  pass PSScriptAnalyzer with `lab/PSScriptAnalyzerSettings.psd1`; the name
  rule applies to the name after `$` or `$script:`.
- **Python.** The checkers under `tools/` and the lab tools under `lab/`
  run on Python 3.11 or later with the standard library alone; their tests
  use pytest. Every public module, class, function and method has a
  Google-style docstring: a one-line imperative summary that ends with a
  period, then, when there is more to say, a blank line, a short
  description and the sections `Args:`, `Returns:` or `Yields:` and
  `Raises:`, each entry `name: what it is`. In tests every module and class
  has a docstring, and a test function has one only when its name cannot
  say what it proves. An `except` names the exceptions it has a recovery
  for. ruff, pyright in standard mode and the code style check are clean;
  fix a finding rather than suppress it, and give any `# noqa` or
  `# pyright: ignore` a reason.

## Validation and testing

CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs the Go,
lint and site jobs and a `CI result` job that fails when any of them failed
or was cancelled, the one status check the ruleset on `main` requires;
[What CI runs](docs/testing.md#what-ci-runs) lists the jobs, the branches,
the runners and the weekly govulncheck workflow.

CI runs a job only for the parts a change touches:
[`.github/classify-changes.sh`](.github/classify-changes.sh) sorts the files
a pull request changed into the categories sh files, Python and Go (Go
taking `docs/timeline-output.md` and `docs/internals.md` too, and Python
this file and the `Makefile`, since tests read them), and runs everything
for a file it does not list. A commit that makes
a job read another file adds that file to the job's category in the
classifier, and the classifier's comments, the workflow's and the table in
[What CI runs](docs/testing.md#what-ci-runs) change with it. A change whose
files set no category is documentation only; locally it needs markdownlint,
the site build and the document and drift checks.

Before committing a change to the program, run from the repository root
`make check` (the Go job's gates in its order), or the commands of the
[Quality gates](#quality-gates) table the change touches. This machine
checks its own platform; `make vet` also vets the Windows and Linux builds,
whose tests run on the CI runners. A change is done when every check on
its pull request, CI and CodeQL, is green and Copilot's review of the final
commit has no unanswered finding, its suppressed and body-only findings
included; watch them with `gh pr checks --watch` and fix what fails before
reporting the pull request ready.

After a catalog change, run a live collection against your own host into a
new private scratch directory, and read the summary:

```sh
make build
SCRATCH=$(mktemp -d)
dist/doubleagent-$(go env GOOS)-$(go env GOARCH) collect -o "$SCRATCH/live" -q
```

Check the error count, the `skipped_excluded` rows for anything new and
large, and that the manifest parses as JSONL. List the `collected` rows not
`secret: true` whose file name contains `key`, `token`, `secret`, `cred`,
`cookie` or `auth`, and account for each: not a credential (say why in the
research document) or a new secret glob. Delete the output afterwards
(`rm -rf "$SCRATCH"`): it holds your own credentials.

### Waiting for long-running checks

- Wait on the process you started: `wait` in the same shell, or each job
  writes its exit code to a file (`...; echo "exit $?" >"$out.rc"`) and the
  wait is for every `.rc` file. Never poll with `pgrep -f` or `ps | grep`,
  which match the waiting shell itself.
- Never wait by searching a log for a line that says the job ended
  (`until grep -q '^ok' "$log"; do sleep 15; done`). A job that ends another
  way, or that prints its result in another form, never writes that line: a
  compiled test binary ends with `PASS` where `go test` prints `ok`, and a
  job that is killed prints nothing. The loop then waits for ever.
- Bound every wait (`timeout 1800 ...`, or a loop of N rounds); a timeout
  is a failure.
- Stop only a process you started, by its process id. Never stop
  processes by name (`pkill`, `killall`, or `kill` on what `pgrep` finds):
  other checks, often another checkout's, run on the same machine under the
  same command names, and a run that is stopped that way reports nothing.
- Read results from the output files, not from whether the watcher ended.
- For CI, use `gh pr checks <n> --watch` or
  `gh run watch <id> --exit-status`.
- While a change is in progress, the tests of the changed packages run
  first (`go test ./internal/<package>`), and the whole suite last, then
  again after each fix.
- A run with no fixed end (fuzzing, generated inputs, a search over
  seeds) gets a wall-clock limit before it starts. Unlike a timeout on a
  check that should finish, reaching the limit is a planned stop, and the
  result says how far the run got.

### Quality gates

Every gate but PSScriptAnalyzer runs in CI: the Go gates in the `go` job,
the site build in the `site` job on the Python the Pages workflow uses,
the others in the `lint` job. The versions are pinned in the workflow's
`env` block, and staticcheck's and govulncheck's also in the `Makefile`;
change them and this table together. `tools/tests/test_version_pins.py`
checks staticcheck and govulncheck only, and fails when their three pins
disagree; the other versions are kept in step by hand. PSScriptAnalyzer, for the lab's
Windows guest scripts, is run by hand. Run the commands from the repository
root before pushing; the `make` targets pass the build tags themselves, and
a `go` command run directly needs `export GOFLAGS=-tags=noasm,nounsafe`
first ([Build tags](internal/AGENTS.md#gates-and-builds)). A
documentation-only change needs markdownlint, the site build and the rows
from document links down.

| Gate | Version | Local command | Configuration |
| --- | --- | --- | --- |
| gofmt | the toolchain `go.mod` pins | `make fmt-check` | none |
| go vet, for Linux, macOS and Windows | the toolchain `go.mod` pins | `make vet` | none |
| go test under the race detector | the toolchain `go.mod` pins | `make race` | none |
| the glob tests against dash, and the JSON depth test | the toolchain `go.mod` pins | `make test-dash`, `make test-nojsonv2` | none; dash must be installed |
| staticcheck | 0.8.2 | `make staticcheck` | none |
| govulncheck | 1.9.0 | `make govulncheck` | none; it needs the network |
| reproducible build | the toolchain `go.mod` pins | `make reproducible` | `.github/go-build.sh` |
| all of the Go gates above, in the CI job's order | | `make check` | |
| ruff lint | 0.16.10 | `uvx ruff@0.16.10 check .` | `ruff.toml`, pydocstyle rules (`D`, `google` convention) included |
| ruff format | 0.16.10 | `uvx ruff@0.16.10 format --check .` | `ruff.toml` (line length 100) |
| pyright, standard mode, Python 3.11 | 1.1.414 | `uvx --from pyright==1.1.414 --with pytest pyright` | `pyrightconfig.json` |
| tests of the checkers and the lab pipeline | pytest, unpinned | `python3 -m pytest tools/tests lab/tests -q` | `tools/tests/conftest.py`, `lab/tests/conftest.py` |
| markdownlint | markdownlint-cli2 0.23.3 | `npx -y markdownlint-cli2@0.23.3 "**/*.md"` | `.markdownlint-cli2.jsonc` |
| site build, strict | the versions `requirements-docs.txt` pins | `uvx --with-requirements requirements-docs.txt mkdocs build --strict -f mkdocs/mkdocs.yml -d "$(mktemp -d)/site"` | `mkdocs/mkdocs.yml`, `mkdocs/hooks.py` |
| shfmt POSIX parse | 3.14.1 | `shfmt -ln posix $(shfmt -f . \| grep -v '^vendor/') >/dev/null` | none; never pass `-w` |
| shellcheck | 0.11.0 | `shellcheck -s sh $(shfmt -f . \| grep -v '^vendor/')` | `.shellcheckrc` |
| checkbashisms | devscripts v2.26.13 | `checkbashisms -p $(shfmt -f . \| grep -v '^vendor/')` | none |
| PSScriptAnalyzer | 1.25.0 | the `pwsh` line of [Installing the linters](docs/development.md#installing-the-linters) | `lab/PSScriptAnalyzerSettings.psd1` |
| code style | none | `python3 tools/check_code_style.py` | none; see [Code style](#code-style) |
| document links | none | `python3 tools/check_doc_links.py` | none; see the link rules under [Conventions](#conventions) |
| research document structure | none | `python3 tools/check_research_structure.py` | the baseline under `tools/drift-baseline/` for the convention rules; see [Research document structure](research/AGENTS.md#research-document-structure) |
| tables against research | none | `python3 tools/check_tables_research.py` | the baseline under `tools/drift-baseline/` |
| parsers against research | the toolchain `go.mod` pins | `python3 tools/check_parser_research.py`, with `go` on the PATH | the baseline under `tools/drift-baseline/` |

- The drift checkers (research document structure and the rows below it)
  compare the research documents with the tables and the parsers. A
  finding missing from `tools/drift-baseline/<agent>.txt` fails, and so
  does a baseline line no finding produces, so an agent's file empties as
  the agent is rebuilt from fresh evidence. `--update-baseline` adds lines
  only in the commit that adds or changes a rule, whose body gives the
  reason; a finding an edit causes is fixed, never baselined.
- Every checker skips `vendor/`: the vendored Go modules are third-party
  source, audited as a dependency ([Go standards](internal/AGENTS.md#go-standards)),
  never held to this project's style, links or pages.
- `shfmt -f .` finds sh files by shebang as well as extension.
- pyright runs with pytest installed, which the checkers' tests import.
- [Installing the linters](docs/development.md#installing-the-linters) says
  how to install the linters that are not on PyPI or npm.

## Conventions

### Git and pull requests

- The project's name is DoubleAgent in prose, titles, the site name and
  descriptions; `doubleagent` is the command, module, repository, every
  path and URL, and stays lowercase in code spans.
- A commit message has an imperative subject under 70 characters and a
  body explaining why.
- Do not use `git stash`. Every working tree of a repository shares one
  stash list, so a `git stash pop` in one checkout can take what another
  checkout put away. To set changes aside, commit them to a branch or save
  them with `git diff > file.patch`; to look at an older commit, add a
  separate working tree (`git worktree add --detach <dir> <commit>`).
- Everything an AI agent writes to git or GitHub (commits, issues, pull
  requests, comments, reviews, review replies), including on a person's
  behalf, discloses AI generation and names the model as the tool reports
  it, both models when a subagent wrote and the main session reviewed. A
  commit ends with `Co-Authored-By: <model name> <noreply address>`
  (`Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`); anything
  else with a line such as `Generated by AI (Claude Opus 5.5).` Nothing
  an agent writes to git or GitHub carries a link to its own session (a
  `claude.ai/code/session_<id>` URL): the reader cannot open it, and it
  binds a public page to an account. This holds over any attribution the
  tool's own reminder asks for: write the body to a file, check it for
  such a line before `gh pr create --body-file`, and never add one.
- Every change reaches `main` through a pull request. A ruleset
  (`gh api repos/seanthegeek/doubleagent/rulesets`) forbids direct pushes,
  force pushes and deletion of `main`, and a merge needs the `CI result`
  job and the CodeQL code-scanning analysis green.
- Work on a branch named for the change; open the pull request with
  `gh pr create` (naming each issue as the issue bullets below say, ending
  with the AI disclosure line), watch `gh pr checks --watch`, and report it
  ready. The user merges it unless the task's instructions said to merge.
  An agent never pushes to `main`, never merges a pull request unasked, and
  never asks for a ruleset bypass.
- Every pull request's base is `main`; the CI `Base branch is main` job
  fails one opened against any other branch. A change that depends on an
  unmerged pull request is folded into it or waits for it to merge and
  rebases; shared files are resolved by rebasing, never by stacking. A
  pull request is reported merged only when
  `git merge-base --is-ancestor <merge commit> main` holds.
- Reproduce a review finding before fixing or declining it. A finding the
  change introduced is fixed in the pull request with a test; a defect in
  untouched code is filed as an issue and fixed in the pull request as the
  issue bullets below say, and the reply names the fix commit, or the issue
  when the work is split off. A fix for one side of a symmetric rule (a
  direction, a scope, a write and its read) re-derives and tests the other
  side in the same commit; when fixing one half of a contract (a flag and
  the summary it feeds, a comment's guarantee and its scope), grep for the
  other half. A second finding that is one more case of the same rule means
  rewriting the rule, not the case: the rule becomes a sentence in the
  function's comment, the code is rewritten from it, and every axis is
  tested before the next push.
- A refactor changes no behaviour: the expected-output tests pass without
  an update. A bug found on the way is fixed in its own commit, with an
  issue.

### Issues

- Track outstanding work as GitHub issues. Anything found while working on
  an issue that the same change can fix (a defect, a gap, an unfinished
  case of the issue's own topic) is filed as its own issue and fixed in the
  same pull request: the fix is its own commit saying `Refs #<n>`, it gets
  its own changelog line ending `([#<n>])` when it changes what the program
  does, and the pull request body lists it under a `Found on the way:` line
  with its `Closes #<n>` line, so it closes and ships with the rest. Only
  work that needs its own plan gets its own issue and pull request, and the
  pull request body names it. Label `bug` behaviour that is wrong, broken
  or contradicts the documentation, and `enhancement` a new feature,
  catalog entry or improvement to documented behaviour.
- An issue filed with `gh issue create` states the version and platform
  as the forms under `.github/ISSUE_TEMPLATE/` ask. A vulnerability is
  never an issue: it goes through the private advisory form named in
  [`SECURITY.md`](SECURITY.md).
- Every bug fixed and feature added has an issue, found among open and
  closed issues or filed before the work is committed. Changes with no
  behaviour of their own (typos, formatting, a test-only refactor) need
  none.
- A commit implementing an issue says `Refs #<n>` in its body, never
  `Fixes` or `Closes`, and each changelog line for it ends with `([#<n>])`.
  GitHub closes an issue for any closing keyword (`close`, `fix`, `resolve`
  and their `-s` and `-d` forms) followed by `#<n>` anywhere in a commit
  message or a pull request body, so in a commit message, and in a pull
  request body outside its `Closes #<n>` lines, write `the fix for #29`,
  never `fixes #29`. The pull request that finishes an issue says
  `Closes #<n>` in its body, one line per issue; a pull request that
  advances an issue without finishing it says `Refs #<n>`. An issue still
  open once the work that finishes it is on `main` (its last pull request
  said `Refs #<n>`, a round settled it, a comment answered it) is closed by
  hand with a comment naming the commit or the answer. No issue waits for a
  release: the changelog entry, not the issue's state, records what a
  version ships.
- Evidence lives in the research documents, not in issues: the commit that
  files or acts on an issue recording file names, keys, field values or
  offsets puts those facts, cited, in `research/<agent>.md`; the issue keeps
  the proposal and a pointer.

### Documentation

- Documentation describes the code, not the intent. Check every sentence
  stating an option, an output file, a field, a value or a behaviour
  against the code before committing, and say where platforms differ and
  what happens without root or Administrator. Never write a sentence the
  code cannot verify. A path is "excluded" only when an `EXCLUDES` glob
  matches it under a collection base a catalog table reaches, giving a
  `skipped_excluded` row in a default run; otherwise it is "not
  collected". A manifest row is described by its independent facts, the
  catalog agent it is attributed to and whether it is flagged
  `secret: true`, never as "collected as secret".
- Document a feature with specifics (an option table row, a file-by-file or
  field-by-field table, the list of values), never in passing. A reader
  with the documentation pages alone must be able to say what each output
  file holds and which command produced it.
- Run a command block the reader will paste as written, in a fresh shell on
  its platform, and write into it what a fresh shell lacks (a PATH entry
  an installer prints but does not apply, an executable the same session
  installed).
- Example output blocks and key, field and column tables are part of the
  interface. A change to anything a command prints or writes (a stdout or
  log line, a manifest field or status, a `collection.json` key, the
  `--list` output, an option, a JSON or JSONL field, a CSV column, a row
  text convention) updates every example and table showing it, in the
  same commit; adding a parser or catalog agent also updates every example
  showing it under `detected but not parsed`, `collected only` or
  `parser: false`, and the [agents page](docs/supported-agents.md) gains a row per new
  agent and a link per parser issue filed or closed, in the commit that adds
  the agent or acts on the issue. Grep the documents for the name, key,
  label or heading. A generated example is regenerated by running its
  command, never edited by hand: the example output blocks of
  [`docs/timeline-output.md`](docs/timeline-output.md) are rewritten by
  `TestTheOutputPageShowsWhatEachExampleCommandPrints`
  ([The output examples](docs/development.md#the-output-examples)), at every
  change that shows in them. A behaviour change without its documentation
  update is incomplete.
- [`docs/differences-from-the-scripts.md`](docs/differences-from-the-scripts.md)
  is the one page that names the earlier collector scripts and Python
  analyzer, and the changelog is the one other page that links it (the
  rules files may). A code comment may cite one of its numbered exceptions
  or differences; no other page mentions the earlier programs.
  `tools/check_doc_links.py` checks the cited numbers and the links.
- The [top-level README](README.md) and [`docs/index.md`](docs/index.md),
  the site's home page listing the pages by group, are overviews only. A
  new page goes under `docs/` and joins that list and the `nav:` tree in
  `mkdocs/mkdocs.yml` beside its siblings, in the group for what the reader
  does with it (Collecting, Analysing, Reference, Contributing); a file that
  is not a page is excluded there, as the `CLAUDE.md` files are.
  `mkdocs build --strict` fails on a document missing from the tree.
  Documents carry no front matter; a page's title is its `# Title`. The
  [documentation site](https://seanthegeek.net/doubleagent/) gives every
  heading the anchor `tools/check_doc_links.py` accepts.
- Documentation addresses its reader, never the agent maintaining it: the
  pages under `docs/` the responder or analyst; `development.md`,
  `internals.md`, `testing.md`, `lab-vms.md`, the lab README and the
  research index a contributor. Only the `AGENTS.md` files hold the rules
  for making a change.
- The `AGENTS.md` and `CLAUDE.md` files state rules, not history. A new rule
  is one or two enforceable sentences in the section that owns it, in the
  file of the area it governs, written as the lesson taught, not the
  incident that taught it: no narrative, no date, and no issue or pull
  request link as its provenance (a live pointer the reader must open, such
  as the issue forms, the ruleset or [`SECURITY.md`](SECURITY.md), is the
  exception). A concrete value (a glob, a name, a row form) stays only when
  the rule depends on it; an example stays only when it is the
  specification; a rule already stated elsewhere is linked, not repeated.
  `tools/check_doc_links.py` fails on an issue or pull request link in any
  of these files. A rule added that a checker can test ships in one pull
  request with that checker extension and a sweep that fixes or baselines
  every existing breach.
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
- The rules' terms of art (cell, round, surface, Observed bullet, listing
  diff, secondary source, nested entry; defined in
  [`lab/AGENTS.md`](lab/AGENTS.md), [`research/AGENTS.md`](research/AGENTS.md),
  [`internal/parsers/AGENTS.md`](internal/parsers/AGENTS.md) and
  [`internal/catalog/AGENTS.md`](internal/catalog/AGENTS.md)) are ordinary
  English elsewhere: a page introduces one before relying on it (a heading
  holding it, a first use linking its definition, a parenthesised gloss) or
  says the thing in plain words. `tools/check_doc_links.py` enforces this
  on every page but the `AGENTS.md` and `CLAUDE.md` files and the research
  documents.

### Links

- A reference from one project document to another is a Markdown link,
  never a bare path or code span: a file as
  ``[`docs/coverage.md`](docs/coverage.md)``, relative to the citing file;
  a section as
  ``[Transcript stores in `research/claude-desktop.md`](../research/claude-desktop.md#transcript-stores)``,
  with the fragment GitHub derives (lower case, punctuation other than
  hyphens removed, spaces to hyphens); one link per section. The
  exceptions are the rules' references by name to the sections of
  [Research document structure](research/AGENTS.md#research-document-structure),
  a template path with a placeholder (`research/<agent>.md`), and a
  catalog path ending in `.md` (an agent's `AGENTS.md` inside a project).
- An external source is one link whose text names it (a page title without
  quotes, `owner/repo issue 105`) and whose target is the full URL with its
  scheme, never a bare `claude.com/docs/...` or a URL as link text. A
  fetch date or note goes in parentheses after the link; a second page on
  the same site is a second full link. Citations to a file at a commit keep
  their form
  ([Validating catalog entries from source](research/AGENTS.md#validating-catalog-entries-from-source)).
- A commit hash in prose, a table cell or a code span is a link,
  `[<short hash>](https://github.com/<owner>/<repo>/commit/<full sha>)`
  (a code span kept inside the link text), with the repository and sha
  from the agent's research document citations; a closed-source binary's
  build hash stays as written.
- An issue or pull request is one link each, such as
  `[issue 92](https://github.com/seanthegeek/doubleagent/issues/92)` or
  `[pull request 129](https://github.com/seanthegeek/doubleagent/pull/129)`;
  a bare `issue <n>`, `PR <n>` or `#<n>` in prose or a table is a defect,
  except the changelog's `([#<n>])` and a number inside a code span.
- `tools/check_doc_links.py`, in the CI lint job, enforces these rules and
  fails on a `section <n>`; run it before pushing a documentation change.

### Changelog, tags and releases

- [`docs/CHANGELOG.md`](docs/CHANGELOG.md) is in Keep a Changelog form:
  work goes under `## [Unreleased]` as it lands, in `Added`, `Changed`,
  `Fixed` and `Removed` subsections, one line per item, written for the
  responder or analyst and calling out any change to the manifest, JSONL or
  CSV interfaces. A documentation-only change needs no line. At a release
  the section becomes `## [x.y.z] - YYYY-MM-DD`, with its comparison link.
- A changelog issue number is a reference link, `([#<n>])`, never a bare
  `(#<n>)`. Its definition,
  `[#<n>]: https://github.com/seanthegeek/doubleagent/issues/<n>`, is
  added with the first line citing the issue, at the bottom after the
  version comparison links, in descending order. markdownlint MD052 (with
  `shortcut_syntax` on) fails on a reference without a definition, MD053 on
  a definition nothing cites.
- The version is one string, set from the tag at release
  ([Builds](internal/AGENTS.md#gates-and-builds)); nothing in the tree is bumped
  by hand. Before a release, re-derive the option, output file and status
  or column lists from the code and diff them against the documentation
  pages.
- Before a tag, every issue closed since the last version tag has one of
  these accounts: a `([#<n>])` line in the changelog, in the pending
  section or in the tagged version it shipped in; a change to nothing the
  program does (documentation, research, tests, development tools), which
  needs no line; or no change at all (an answered question, a round that
  settled it, a close as not planned or duplicate). List them with

  ```sh
  gh issue list --state closed --limit 1000 --search "closed:>=$(git for-each-ref \
    --format='%(*committerdate:iso-strict)' "refs/tags/$(git describe --tags --abbrev=0 --match 'v*')")"
  ```

  The count must stay under the limit. In the other direction, every issue
  a `([#<n>])` line of the pending section cites is closed.
- A release requires zero open Dependabot alerts, which CI cannot check.
  Before tagging, run
  `gh api "repos/$(gh repo view --json nameWithOwner -q .nameWithOwner)/dependabot/alerts?state=open" -q length`
  and resolve or dismiss, with a reason, each alert it counts.
- Tagging is the user's decision. An agent never creates or pushes a tag
  unless the user asked for that tag in the current session; a finished
  changelog or green CI is not that permission. Once the changelog's
  version section has reached `main` and the user has said to tag it, tag
  it `vX.Y.Z` (a pre-release `vX.Y.Z-rc.N`) with an annotated tag and push
  the tag; the release workflow fails unless `main` holds the tagged commit
  and the tag has no release yet, then makes a draft release, which the user
  reads and publishes ([Releases](docs/development.md#releases)).
- A tag marks a change to what the program does. A change touching only
  documentation, research documents, the `AGENTS.md` files or the linter
  configuration gets no tag of its own; a pending tag may sit on it or on
  the last code commit, as the user prefers.

## Things to avoid

- Parsing transcripts on the host, or giving `collect` a dependency the
  host must provide.
- Collecting model weights, editor caches or extension binaries by default.
- Reading files through symlinks or descending into symlinked directories.
- Writing to `/tmp` by default or anywhere other than `-o`.
- Interactive prompts, `sudo` calls, or anything that reads stdin.
