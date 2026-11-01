# CLAUDE.md

@AGENTS.md

Notes for Claude Code on this workstation:

- After editing a collector, run `collectors/tests/smoke.sh dash` first: dash
  is the strictest installed shell. After editing the analyzer, run its
  tests in the order
  [Waiting for long-running checks](AGENTS.md#waiting-for-long-running-checks)
  gives, with the venv below. Run the matching
  [Quality gates](AGENTS.md#quality-gates) before committing; CI runs them,
  the pre-commit hook does not.
- Pass `-q` to the collector from a tool call; without it, every collected
  path is logged to stderr.
- Run `tools/diff_smoke_rows.py` with `--shell dash`: it runs the collector
  twice, and its default `/bin/sh` is the slow bash 3.2 here. Never run it
  beside a VM or another smoke test.
- A live collection here gathers the author's own agent credentials: write
  it under the scratchpad and delete it afterwards.
- This Mac's `python3` is 3.14 without `zstandard` or `python-dateutil`, so
  `analyzer/tests/run.sh` fails at import. Make a venv in the scratchpad
  (`python3 -m venv venv && venv/bin/pip install -r analyzer/requirements-dev.txt
  ruff==0.16.10 pyright==1.1.414`), run the suite from `analyzer/` with
  `venv/bin/python -m pytest`, and pyright with
  `--pythonpath venv/bin/python`. Run `tools/check_fixture_schema.py` and
  `-m pytest tools/tests` from the repository root with that venv's
  python. `uv` (Homebrew) installs any CPython the analyzer supports: to run
  the suite under the minimum version, `uv python install 3.11`, then
  `uv venv --python 3.11 venv311 && uv pip install --python venv311/bin/python
  -r analyzer/requirements-dev.txt`.
- This Mac has `/bin/dash` and `uv` with `uvx`, but no busybox, posh, pwsh, shellcheck or
  shfmt; CI runs those gates. Its `/bin/sh` is bash 3.2, the one shell CI
  does not run; the smoke test under it takes about 17 minutes with no VM
  running and far longer beside a Parallels VM, which holds most of the
  8 GB, so never run it beside one. While a Linux lab VM is up, run dash,
  bash and busybox there and the analyzer tests here.
- A Bash call with `run_in_background` and a subagent each re-invoke the
  session when they end, with the exit code and the output file: wait for
  that notification and read the file. Never write a loop that sleeps and
  polls for the run (`until ! pgrep -f 'smoke.sh dash'`, `while ps | grep`),
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
  bug fix, a small feature, a smoke fixture with its checks, a changelog or
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
