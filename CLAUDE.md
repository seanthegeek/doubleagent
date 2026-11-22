# CLAUDE.md

@AGENTS.md

Notes for Claude Code on this workstation:

- After editing the binary, run the Go commands of
  [Validation and testing](AGENTS.md#validation-and-testing) with
  `GOFLAGS=-tags=noasm,nounsafe`; the `dash` tag's glob test uses
  `/bin/dash`. After editing a checker or the round pipeline, run their
  tests with the venv below. Run the matching
  [Quality gates](AGENTS.md#quality-gates) before committing; CI runs them.
- Pass `-q` to `doubleagent collect` from a tool call; without it, every
  collected path is logged to stderr.
- A live collection here gathers the author's own agent credentials: write
  it under the scratchpad and delete it afterwards.
- This Mac's `python3` is 3.14 without pytest. Make a venv in the scratchpad
  (`python3 -m venv venv && venv/bin/pip install pytest pytest-xdist
  ruff==0.16.10 pyright==1.1.414`) and run `-m pytest tools/tests
  lab/round/tests` from the repository root with that venv's python, and
  pyright with `--pythonpath venv/bin/python`. `uv` (Homebrew) installs any
  CPython: to run the tests under the minimum version, `uv python install
  3.11`, then `uv venv --python 3.11 venv311 && uv pip install --python
  venv311/bin/python pytest`.
- This Mac has `/bin/dash` and `uv` with `uvx`, but no pwsh, shellcheck or
  shfmt; CI runs the sh linters. Its `/bin/sh` is bash 3.2. The lab VMs can
  run together and beside local test runs.
- A Bash call with `run_in_background` and a subagent each re-invoke the
  session when they end, with the exit code and the output file: wait for
  that notification and read the file. Never write a loop that sleeps and
  polls for the run (`until ! pgrep -f 'go test'`, `while ps | grep`),
  in this session or in a brief: `pgrep -f` matches the waiting shell
  itself, so the loop never ends. Bound a foreground command with
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
  parallel. `model: sonnet` for a routine, well-scoped unit: one module, a
  bug fix, a small feature, a test fixture with its checks, a changelog or
  agents-page update. `model: opus` for a large multi-file unit (a research
  document rewrite, the rebuild of an agent's tables, parser, fixtures and
  tests from its document, a new parser with its fixture and tests, a
  checker with its tests, the integration that merges the pieces) or for a
  unit that is complex though contained: a join predicate, a reader for a
  specified format, a protobuf or SQLite decoder, a paired write and read
  where one side is easy to get silently wrong. The model is decided at
  planning time; when in doubt, Opus. When units touch the same files, give
  each `isolation: worktree`, have it commit on its branch without pushing,
  keep shared-file edits append-only, and send one more subagent, on the
  model of the larger unit, to merge the branches and resolve the overlaps
  (registries, fixtures, test classes, README tables).
- **Fable reviews.** The main session checks each diff against the brief
  and the research documents, runs the tests itself, fixes or sends back
  what is wrong, then commits, pushes and opens the pull request; the user
  merges and tags ([Conventions](AGENTS.md#conventions)). A subagent's
  report is evidence, not approval: a claim that tests pass is re-run. A
  pull request review (Copilot's findings, a cold reader's) is answered by
  Fable too.
- **Opus reads it cold.** After the main session commits and before it
  opens the pull request, one Opus subagent with nothing from the session
  reviews the committed diff from a bare prompt: read AGENTS.md, read every
  changed file whole, run the gates the change needs under the notes
  above, ask whether the hunks agree with each other, label each
  finding substantive or wording. A substantive finding is fixed and the
  round repeated until one finds only wording. Copilot's review on the pull
  request is the second cold reader, never the first.
- **A review loop has a stated end.** The brief of each round of a loop of
  cold reviews names its scope (the commits no reviewer has read), what
  counts as must-fix (a crash, a hang, reading outside the collection, wrong
  output on evidence an agent really writes, cost growing faster than the
  input) and that a round with no must-fix ends the loop. Once a loop
  runs, a finding outside that definition becomes an issue, never a new
  unit, and no new class of work starts without the user's word.

The brief is one file in the scratchpad that every subagent of a round
reads, the cold reader excepted, so the rules are identical and each
prompt is only its assignment.
It names the files to read first, the rules that must hold (the waiting
rule above among them), the files the agent may and may not touch, and the
report format.

In a [real-install round](AGENTS.md#real-install-rounds) of a CLI agent,
Fable drives the agent in the VM over ssh and `screen`; the user does only
the browser, phone and GUI steps.

Opus is the fallback for the planning and review steps only when Fable is
unavailable. If Fable becomes unavailable mid-task (a usage-limit or
rate-limit error), say so to the user in plain words first, then run the
planning and review steps through `model: opus` subagents and label the
result Opus-reviewed, so the user can decide whether a Fable pass is still
wanted. Never
fabricate a report from a subagent that was cut off; relaunch it.
