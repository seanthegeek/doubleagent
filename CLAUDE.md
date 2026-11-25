# CLAUDE.md

@AGENTS.md

Each area directory (`internal/`, `internal/catalog/`, `internal/parsers/`,
`research/`, `lab/`) has a `CLAUDE.md` that imports its `AGENTS.md`, and
those of `internal/parserkit/` and `internal/growthtests/` import the
parser rules in `internal/parsers/AGENTS.md`; Claude Code loads a
directory's `CLAUDE.md` when it reads a file there. A subagent's brief names
the area `AGENTS.md` files its work needs, since a subagent that never reads
a file in an area never loads that area's rules.

Notes for Claude Code on this workstation:

- Go is installed under `/usr/local/go/bin`, which a tool call's shell does
  not have on its `PATH`: run `export PATH=/usr/local/go/bin:$PATH
  GOFLAGS=-tags=noasm,nounsafe` first, or a `make` target, which passes the
  tags itself. After editing the program, run the tests of the changed
  packages, then `make check` or the Go rows of
  [Quality gates](AGENTS.md#quality-gates). This Mac runs its own
  platform's tests; the Windows and Linux code is only vetted here
  (`make vet`), and its tests run on the CI runners or a lab VM.
- Pass `-q` to `doubleagent collect` from a tool call; without it, every
  collected path is logged to stderr.
- A live collection here gathers the author's own agent credentials: write
  it under the scratchpad and delete it afterwards.
- This Mac's `python3` is 3.14 without pytest. Make a venv in the scratchpad
  for the checkers under `tools/` and the lab tools under `lab/`
  (`python3 -m venv venv && venv/bin/pip install pytest ruff==0.16.10
  pyright==1.1.414`) and run `-m pytest tools/tests lab/tests` from the
  repository root with that venv's python, and pyright with
  `--pythonpath venv/bin/python`. `uv` (Homebrew) installs any CPython: to
  run the tests under the minimum version, `uv python install 3.11`, then
  `uv venv --python 3.11 venv311 && uv pip install --python
  venv311/bin/python pytest`.
- This Mac has `/bin/dash` (the `make test-dash` run uses it), `uv` with
  `uvx`, and Homebrew's shellcheck and shfmt at the pinned versions, but no
  pwsh or checkbashisms; CI runs checkbashisms, and PSScriptAnalyzer
  runs on the Windows lab VM. Its `/bin/sh` is bash 3.2. The lab VMs can run
  together and beside local test runs.
- A Bash call with `run_in_background` and a subagent each re-invoke the
  session when they end, with the exit code and the output file: wait for
  that notification and read the file. Never write a loop that sleeps and
  polls for the run (`until ! pgrep -f 'go test'`, `while ps | grep`), in
  this session or in a brief: `pgrep -f` matches the waiting shell itself,
  so the loop never ends. Bound a foreground command with
  `perl -e 'alarm N; exec @ARGV' -- <command>`, since macOS has no
  `timeout`.
- Under fish, a string of ssh flags in a variable is not word-split: keep an
  ssh config file with host aliases in the scratchpad and pass it with `-F`.

## Orchestration

Work is split by model to keep Fable for the parts that need it.

- **Fable plans.** The main session scopes the task and reads what it
  needs. For a new feature or a change to one, it enters plan mode, designs
  the change, presents the plan to the user, and starts nothing until the
  user approves or amends it. Then it writes the brief and splits the work,
  naming each unit's model in the brief.
- **Sonnet implements by default; Opus for large or complex work.** Every
  implementation step goes to a subagent, one per self-contained unit, in
  parallel. `model: sonnet` for a routine, well-scoped unit: one package, a
  bug fix, a small feature, a fixture image entry with its coverage, a
  changelog or agents-page update. `model: opus` for a large multi-file unit
  (a research document rewrite, the rebuild of an agent's catalog lines,
  parser, fixtures and tests from its document, a new parser with its tests
  and growth cases, a checker with its tests, the integration that merges
  the pieces) or for a unit that is complex though contained: a join
  predicate, a reader for a specified format, a protobuf or SQLite decoder,
  a paired write and read where one side is easy to get silently wrong. The
  model is decided at planning time; when in doubt, Opus. When units touch
  the same files, give each `isolation: worktree`, have it commit on its
  branch without pushing, keep shared-file edits append-only, and send one
  more subagent, on the model of the larger unit, to merge the branches and
  resolve the overlaps (the parser registry, the growth-case list
  `growthSets()` in `internal/growthtests/growth_test.go`, the fixture image,
  expected-output files, the agents page).
- **Fable reviews.** The main session checks each diff against the brief
  and the research documents, runs the tests itself, fixes or sends back
  what is wrong, then commits, pushes and opens the pull request; the user
  merges and tags ([Conventions](AGENTS.md#conventions)). A subagent's
  report is evidence, not approval: a claim that tests pass is re-run. A
  pull request review (Copilot's findings, a cold reader's) is answered by
  Fable too.
- **Opus reads it cold.** After the main session commits and before it
  opens the pull request, one Opus subagent with nothing from the session
  reviews the committed diff from a bare prompt: read the root `AGENTS.md`
  and the `AGENTS.md` of every area the diff touches, read every changed
  file whole, run the gates the change needs under the notes above, ask
  whether the hunks agree with each other, label each finding substantive
  or wording. A substantive finding is fixed and the round repeated until
  one finds only wording. Copilot's review on the pull request is the second
  cold reader, never the first.
- **A review loop has a stated end.** The brief of each round of a loop of
  cold reviews names its scope (the commits no reviewer has read), what
  counts as must-fix (a crash, a hang, reading outside the collection, wrong
  output on evidence an agent really writes, cost growing faster than the
  input, each shown by a run and reached as
  [Proof before a fix](AGENTS.md#issues) says) and that a round with no
  must-fix ends the loop. Once a loop runs,
  a finding outside that definition becomes an issue, never a new unit, and
  no new class of work starts without the user's word.

The brief is one file in the scratchpad that every subagent of a round
reads, the cold reader excepted, so the rules are identical and each
prompt is only its assignment. It names the files to read first (the area
`AGENTS.md` files among them), the rules that must hold (the waiting rule
above among them), the files the agent may and may not touch, and the
report format.

In a [real-install round](lab/AGENTS.md#real-install-rounds) of a CLI
agent, Fable runs the lab pipeline, `lab/lab.sh`, which drives the agent in
the VM; the user does only the browser, phone and GUI steps.

Opus is the fallback for the planning and review steps only when Fable is
unavailable. If Fable becomes unavailable mid-task (a usage-limit or
rate-limit error), say so to the user in plain words first, then run the
planning and review steps through `model: opus` subagents and label the
result Opus-reviewed, so the user can decide whether a Fable pass is still
wanted. Never fabricate a report from a subagent that was cut off; relaunch
it.
