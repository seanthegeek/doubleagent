# lab/AGENTS.md

The rules for real-install rounds and for the lab pipeline that runs them.
The rules every change keeps are in the [root `AGENTS.md`](../AGENTS.md);
a round's findings go into research documents under
[`research/AGENTS.md`](../research/AGENTS.md), and the catalog and parser
changes it settles follow
[`internal/catalog/AGENTS.md`](../internal/catalog/AGENTS.md) and
[`internal/parser/AGENTS.md`](../internal/parser/AGENTS.md).
[`lab/README.md`](README.md) says how to run the pipeline, and
[`docs/lab-vms.md`](../docs/lab-vms.md) describes the VMs.

## Real-install rounds

A round installs one agent in a clean VM, never on the workstation,
exercises it, and compares what it wrote with the research documents, the
catalog and the parser. A surface is one form the product ships in (a CLI,
a desktop app, an editor fork, an extension, a packaged build beside a
direct download), and a cell is one surface on one platform.

A round covers every surface the agent ships on every platform it ships it
for; each cell is its own round, and has one Observed bullet, kept current
([Research document structure](../research/AGENTS.md#research-document-structure)).
A round is required, before any table, document or parser changes, when an
agent is added to the catalog, when its coverage is extended (a cell
without an Observed bullet, a feature the documents do not mention), and
when a version newer than the last Observed bullet's is in use. A newer
build is a new round of its cell, never a confirmation of the old one: it
rewrites the bullet for that build, moves the older build into the
bullet's earlier-builds sentence, and moves what the older build wrote
differently into Format versions.

1. **Survey names first, one step at a time.**
   - A CLI agent is driven by the pipeline, `lab/lab.sh`, from its step
     table under `lab/agents/<agent>/`: the main session runs it, and the
     person does only what needs a browser, a phone or the VM's GUI, and
     says when it is done. A browser step is the person's step, never a stop
     point: it is completed, with a throwaway account where one exists, not
     cancelled. A GUI agent is the person's to drive.
   - After each step a listing of the agent's directories is taken (names,
     sizes, modes and modification times only, by the survey script the
     pipeline copies into the VM's lab directory) and diffed against the
     previous one before the next step; the diffs (listing diffs) are the
     Observed bullet's evidence. A round that ran every task before the
     first listing says so in its document and gives the modification
     times.
   - Listing names sort in the order taken: zero-padded step numbers, an
     unplanned event taking the next number, never a letter.
   - The survey's roots are fixed before the baseline and include every
     surface the round will touch; a root added mid-round names its first
     listing as its baseline. The survey also counts the agent's processes
     by name.
2. **Read shapes, never values.** `lab/tools/shape.py` prints key names,
   value types, JSONL line types, byte totals and the enum tokens of a fixed
   set of paths, every value through its redaction function, so a UUID, a
   hex id, a path or a URL prints as its class; a new printing path gets a
   test feeding it those four forms. Run it with `python3 -I`; write no
   ad-hoc script that prints values. Nothing from a real install goes into
   the repository, an issue or a report beyond file names, key names, types,
   sizes, file modes, modification times, enum values, credential-store item
   names (the service, and the account as its class, such as the OS user)
   and log lines with ids redacted.
3. **Exercise every surface separately.** Each item is its own step with its
   own listing diff, run for every kind of task the surface offers.
   - Before the baseline, check the step table against this list and the
     previous cell's lessons, and fix it. Before the first prompt, record
     the REPL's `?` shortcut list and `/help`.
   - Before its first use, every slash command the round uses is typed with
     a trailing space and nothing else (`/auth `), as its own step whose
     screen saves the agent's completion list of subcommands and their
     descriptions, and the line is cleared before the next step.
   - The first prompt is a question the model answers without a tool
     (`what is 17 + 25?`), so the round records a turn with no tool call; if
     the agent calls one, the repeat comes before any other prompt.
   - A prompt that needs a web search (a fact newer than the model can know,
     such as the latest release of a named package), so the round records
     how the agent stores a call of its own web search or fetch tool and its
     results. A `curl` through the shell is a missed prompt, kept and
     repeated as the missed-prompt rule below says.
   - A prompt likely to make the agent ask before acting (a task naming no
     file, a request missing a detail it needs), so the round records how
     the agent stores a turn in which it asks the user and waits; an agent
     that acts without asking is a finding, and the repeat comes with a
     sharper prompt as the missed-prompt rule below says.
   - A session with a tool call, a task that writes files, a scheduled or
     automated task, a plugin or MCP server, a resumed session, and a short
     and a long paste as prompts, each paste its own step.
   - One tool call approved, one denied, an "allow always" or equivalent
     grant, and a model switch mid-session. Record the default permission
     mode at the first tool call; if it auto-approves, switching to the
     asking mode is its own step, with the mode names in cycle order.
   - At every dialog, move through each option and read the screen after
     each move before confirming; option wording and footer hints
     (`Tab to amend`, `Esc to cancel`) are findings and the way to the extra
     options (a reason field, a directory-scoped grant).
   - A session synced to the vendor's account where the agent offers it (a
     resume, a sign-in or a setting may do it unasked), with a listing diff
     showing which files mark it.
   - A source-host integration (GitHub, GitLab, Bitbucket) where offered,
     with a throwaway account: connect through the agent's own integration
     and, in an editor, the editor's provider too; run a remote agent with a
     read-only task ("describe this repository, change nothing, open no pull
     request"); reopen that conversation after a relaunch. Record whether
     the account is named on disk, where its token or grant lives (a file,
     the editor's secret storage, the OS keychain, or server-side only),
     whether the repository list is cached, and whether repository-bound
     conversations are stored, cached or fetched.
   - A source-host CLI the agent leans on (`gh`) is logged in first, as its
     own step, with a listing showing where its token landed (its plaintext
     `hosts.yml` over SSH, the OS keychain from the GUI), then the agent's
     own flow is completed. Log `gh` in with `gh auth login --expiring`
     (gh 2.103.0 or later), only for steps that act on the public lab
     repository and the one-time store observation below, and run
     `gh auth logout` before the VM is released; the logout removes only the
     local token, and the grant itself is revoked under the account's
     authorized OAuth apps.
   - An agent driven over SSH may fail to reach the OS credential store and
     write a file fallback instead, and whether it fails can turn on a
     desktop session being logged in at the time. On every platform, record
     where its credentials land at the ssh sign-in (the credential entered
     in the session started over SSH): where the agent takes a key at a
     prompt, the pipeline types it from its credentials file before anyone
     has logged in to the VM's desktop since the clean snapshot; where the
     sign-in has a browser part, the person attaches to that session from
     the VM's desktop, and the research document says a desktop session was
     logged in. Then sign out, as its own step with a listing; then the
     person makes the desktop sign-in (the agent's own sign-in from a
     terminal of the VM's desktop), with a listing after, once the last step
     driven over SSH is done, except the steps of the store-or-fallback
     variable the next bullet names. The sign-out is the agent's own command
     or dialog option, taken from its completion list or its research
     document and run over SSH; where the agent has none, the person removes
     the stored credential the way a user would, and the research document
     says the agent offers no sign-out. A source-host CLI the agent leans on
     follows the same order, once per tool and platform; later cells cite
     the research document.
   - Every environment variable or dotfile the research document names that
     moves a store, adds one or decides whether a credential is written gets
     its own steps: a start with it set, one tool-free question, a stop, and
     a listing diff, its path among the survey roots. A key-carrying one
     takes its value from the pipeline's credentials file through a token
     the step log never expands, never typed into the step table. A variable
     that decides between the OS credential store and a file fallback is
     exercised over SSH after the desktop login, when the store is
     reachable. A variable that only changes what the model is told (a
     system-prompt file) is out.
   - A session deleted the way a user would (the agent's own delete or
     clear, or the project directory removed). When the delete offers to
     remove the synced copy, take it and record whether a local file still
     marks the sync.
   - A set idle period with a listing, then a reboot and a listing after
     login.
   - A surface that writes nothing locally is a finding, and so is a feature
     the VM cannot run; say which. A feature this account or build lacks is
     recorded as not exercised, with the reason.
   - The step table carries the exact prompt per step; a step whose prompt
     misses is kept as a finding about the agent, and repeated with a
     sharper prompt under the next name. An install command is copied from
     the vendor's page or script, never retyped, and pipes to the shell its
     shebang names. The person reports what the screen showed at each step.
4. **Run `doubleagent collect` in the VM** as the user, before and after the
   delete steps (the pipeline's `collect` action), and pull the manifests,
   summary, log and archive. Read the error rows, the `skipped_excluded`
   rows and the unflagged-key list of
   [Validation and testing](../AGENTS.md#validation-and-testing), diff the
   two manifests, and check that `doubleagent timeline` gives every observed
   session rows and no `system` row reports an unreadable file. A parser
   update starts by diffing the shape tool's line types and key sets against
   the research document's. The step table pairs each step with the claim
   it tests; the evidence row records settled, contradicted or new. A
   shape-tool change lands with its tests before the next listing. An ad hoc
   check (a listing diff, a row count, a grep) expecting matches fails on
   zero; one expecting none is first run against a known match, since a
   filter aimed at the wrong path returns an empty, passing-looking result.
5. **Write it down the same day.**
   - The cell's Observed bullet is written, or rewritten, for this round:
     version, build, date, what was exercised and what each surface wrote. A
     layout the parser handles that no real install has shown is marked "not
     observed on any real install, bundle-derived".
   - Every statement about a product surface (a tab, a button, a mode)
     carries the build and date; every claim about what an agent writes,
     sanitises or removes names the build or commit it was verified at.
   - What the delete left and removed goes into
     `#### What survives a delete` and `#### Deletion rows`, linked from the
     Observed bullet. Findings about `doubleagent` go to an issue or the docs
     page they concern, never the research document.
   - Catalog and parser changes follow the usual checklists, under the
     changelog's `Unreleased` section.
6. **Clean up, after the main session says so.**
   - The main session says, in plain words and as its own line, when the VM
     may be reverted: only after the last listing, with the collection and
     probe output on the analyst machine and its outputs in the VM deleted.
     Until then it says the VM is still needed. The pipeline's `finish`
     action then reverts the VM to the clean snapshot and deletes the
     snapshots the pipeline took, but deletes no file: the person deletes
     the cell directory, which holds the pulled collections, before the next
     agent.
   - Automatic updates and reboots are off in the clean snapshot. The
     pipeline snapshots the VM at every pause step; a hard-to-undo step (a
     delete, a retention sweep, a reboot) that no pause precedes gets a
     snapshot from the main session (`prlctl snapshot <vm> -n <name>`)
     before it runs, or a pause step in the step table.
   - A revert to the snapshot after step N discards every later listing;
     those steps are retaken under new names, and the evidence table says
     which snapshot and why. Mid-round snapshots are deleted when the VM is
     released, a snapshot taken by hand by whoever took it, since `finish`
     deletes only the pipeline's; the clean snapshot is kept.

Subagents integrate from one brief file holding every number the documents
will cite, the install facts, the collection and timeline runs, and the
assignments. They never run `doubleagent collect` against the VM or read its
user's data, and the main session posts to GitHub. Two run in parallel on
disjoint files: one for the collection side (catalog lines, discovery, the
fixture image, the collection pages, the changelog), one for the research
document and the agents page; each reports every check it ran with the
result line.

The main session then re-runs the checks (`make check`, markdownlint, the
document link check and the drift checkers of
[Quality gates](../AGENTS.md#quality-gates)), fixes each drift finding the
round's evidence settles and removes its baseline line, runs
`doubleagent timeline` over the round's archive with the merged parser,
compares each new row form's count and the test records' values with the
real records, reviews the diffs against the brief, and commits. When the
research and parser subagents run in parallel, the Parser plan's row forms
are copied from the parser's package comment at review.
