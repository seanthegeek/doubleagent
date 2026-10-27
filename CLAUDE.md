# CLAUDE.md

@AGENTS.md

Notes for Claude Code on this workstation:

- After editing a collector, run `collectors/tests/smoke.sh dash` first: dash
  is the strictest installed shell. After editing the analyzer, run
  `analyzer/tests/run.sh`. Run the matching
  [Quality gates](AGENTS.md#quality-gates) before committing; CI runs them,
  the pre-commit hook does not.
- Pass `-q` to the collector from a tool call; without it, every collected
  path is logged to stderr.
- A live collection here gathers the author's own agent credentials: write
  it under the scratchpad and delete it afterwards.
- This Mac's `python3` is 3.14 without `zstandard` or `python-dateutil`, so
  `analyzer/tests/run.sh` fails at import. Make a venv in the scratchpad
  (`python3 -m venv venv && venv/bin/pip install -r analyzer/requirements.txt
  ruff==0.16.10 pyright==1.1.414`), run the suite from `analyzer/` with
  `venv/bin/python -m unittest discover -s tests -t tests`, and pyright with
  `--pythonpath venv/bin/python`.
- This Mac has `/bin/dash` but no busybox, posh, pwsh, uvx, shellcheck or
  shfmt; CI runs those gates. Its `/bin/sh` is bash 3.2, the one shell CI
  does not run; the smoke test under it takes about 17 minutes with no VM
  running and far longer beside a Parallels VM, which holds most of the
  8 GB, so never run it beside one. While a Linux lab VM is up, run dash,
  bash and busybox there and the analyzer tests here.
- Under fish, a string of ssh flags in a variable is not word-split: keep an
  ssh config file with host aliases in the scratchpad and pass it with `-F`.

## Orchestration

Work is split by model to keep Fable for the parts that need it.

- **Fable plans.** The main session scopes the task, reads what it needs,
  writes the brief and splits the work.
- **Opus implements.** Every implementation step (a parser, a research
  document, a citation pass, the integration that merges the pieces) goes
  to a subagent with `model: opus`, one per self-contained unit, in
  parallel. When units touch the same files, give each
  `isolation: worktree`, have it commit on its branch without pushing, keep
  shared-file edits append-only, and send one more Opus subagent to merge
  the branches and resolve the overlaps (registries, fixtures, test
  classes, README tables).
- **Fable reviews.** The main session checks each diff against the brief
  and the research documents, runs the tests itself, fixes or sends back
  what is wrong, then commits, pushes and opens the pull request; the user
  merges and tags ([Conventions](AGENTS.md#conventions)). A subagent's
  report is evidence, not approval: a claim that tests pass is re-run.

The brief is one file in the scratchpad that every subagent of a round
reads, so the rules are identical and each prompt is only its assignment.
It names the files to read first, the rules that must hold, the files the
agent may and may not touch, and the report format.

In a [real-install round](AGENTS.md#real-install-rounds) of a CLI agent,
Fable drives the agent in the VM over ssh and `screen`; the user does only
the browser, phone and GUI steps.

If Fable becomes unavailable mid-task (a usage-limit or rate-limit error),
say so to the user in plain words first, then run the planning and review
steps through `model: opus` subagents and label the result Opus-reviewed,
so the user can decide whether a Fable pass is still wanted. Never
fabricate a report from a subagent that was cut off; relaunch it.
