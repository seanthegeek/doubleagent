# Round pipeline

`round.sh` runs a real-install round of one terminal agent on one lab VM
from the analyst's Mac: a
[real-install round](../../AGENTS.md#real-install-rounds) installs an
agent in a clean VM, exercises it one step at a time, lists what it wrote
after every step, collects and analyzes, and reverts the VM. One agent on
one VM is a cell (one platform of one product form, in the round's terms),
and each run of `round.sh` works on one cell directory. Use it instead of
driving the agent by hand over `ssh` and `screen` whenever the agent is a
terminal program; a GUI agent is still driven by the person.

The pipeline drives the agent through
[microsoft/tui-test](https://github.com/microsoft/tui-test), a headless
terminal whose sessions live in a daemon between calls, so each step is
one `ssh` call that types, presses keys or reads the rendered screen.
It stops for the person at every step that needs a browser, a phone or
the VM's desktop, and picks up where it stopped.

## Running a cell

```sh
lab/round/round.sh gemini-cli Ubuntu ~/rounds/gemini-cli-ubuntu start
# ... exit 3: a person step; do it, then
lab/round/round.sh gemini-cli Ubuntu ~/rounds/gemini-cli-ubuntu resume
lab/round/round.sh gemini-cli Ubuntu ~/rounds/gemini-cli-ubuntu collect
lab/round/round.sh gemini-cli Ubuntu ~/rounds/gemini-cli-ubuntu finish
```

The arguments are the agent (a directory under `agents/`), the VM (its
name in `prlctl list -a`, with a file under `vms/`) and the cell
directory, which must lie outside the repository because it receives the
agent's credentials with the collection.

| Action | What it does |
| --- | --- |
| `start` | Refuses when another VM is running or the cell has started. Copies the agent's `steps.tsv` and `roots` into the cell, downloads the pinned tui-test release and checks its sha256, reverts the VM to its clean snapshot, starts it and waits for `ssh`, copies the survey and drive scripts, `analyzer/tools/shape.py` and this checkout's collector into `~/lab` in the VM, makes the sample project `~/lab/proj`, takes listing `00-baseline`, runs the agent's install script, takes `01-install`, then runs the steps. |
| `resume` | Finishes the paused person step (its screen and listing), then runs the remaining steps. |
| `collect` | Runs the collector in the VM as the lab user, pulls its archive, manifest, summary, log and hash file into the cell, deletes them in the VM, then runs the analyzer and the shape tool on the Mac and rewrites the report. |
| `finish` | Stops the agent's session, takes the last listing, stops the VM, reverts it to the clean snapshot, deletes the snapshots this cell took and only those, and prints what is left for the person. |
| `status` | Prints the cell's state and step log. |

Exit status 3 means a person step is waiting; 1 is an error, with the
reason on stderr and in `round.log`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `ROUND_PYTHON` | `python3` | The Python that runs the analyzer: one with `analyzer/requirements.txt` installed. The pipeline's own tools need only the standard library. |
| `ROUND_KNOWN_HOSTS` | `<cell>/known_hosts` | The known-hosts file of the cell's ssh config. |
| `ROUND_SSH_KEY` | `~/.ssh/doubleagent_lab` | The lab key, as [SSH access in Lab VMs](../../docs/lab-vms.md#ssh-access) describes it. |
| `ROUND_STOP_AFTER` | none | A step number after which `start` or `resume` stops with exit 0; `resume` goes on from the next step. |
| `TURN_SECONDS`, `QUIET_SECONDS` | the agent's `agent.conf` | How long a step waits for its expectation, and how long the screen must stay unchanged to count as settled. |
| `PRLCTL` | `prlctl` | The Parallels Desktop command line. |

## What the person does

A `pause` step prints the agent's screen and the step's text, takes a VM
snapshot named `claude-<agent>-<NN>` and exits 3. The person does what
the text says and runs `resume`, which records the screen and the
listing after the person's step before going on.

A sign-in happens inside the agent's own terminal session. In the VM's
desktop, open a terminal and attach to it:

```sh
~/lab/tui-test/tui-test --session s02 monitor --interactive
```

The session name is in the pause text. This is how the person joins the
session the pipeline started over `ssh`. Complete the agent's sign-in
with the throwaway account, the browser part included, then detach with
Ctrl+] and run `resume`. If attaching fails, sign in from a desktop
terminal of your own (start the agent there, sign in, exit) and run
`resume`: the next listing records where the credential landed either
way. The pipeline never asks for, stores or prints a credential; whatever
file the agent writes for it shows in the next listing by name, size and
mode.

## The step table

`agents/<agent>/steps.tsv` holds one step per line, its fields separated
by tabs and none empty: `NN`, `name`, `kind`, `text`, `expect` and `claim`.
Lines starting with `#` and the header line are skipped. `NN` is two or
three digits rising from `02` (`00-baseline` and `01-install` are the
listings `start` takes itself), and `NN-name` names the step's listing,
diff and screen files. `claim` is the statement of the agent's research
document the step tests, with its section anchor, or `none`. A claim
ending `(unverified)` rests on a guessed value (a dialog's option order, a
key, a command name, the input marker) that the first run of the table
confirms or corrects.

Each table follows the step list of
[Real-install rounds in AGENTS.md](../../AGENTS.md#real-install-rounds).
An item is left out of an agent's table when its research document says
the agent lacks that feature, with no step to record the absence:
Gemini CLI has no scheduled task, no session synced to an account and no
source-host integration; Qwen Code syncs no session to an account (its
channel sessions are relays through a messaging platform); pi has no
permission modes, no scheduled task, no synced session (`/share` is left
out too) and no source-host integration. A resumed session after a
relaunch is the same terminal program for all three, so the `/exit` and
resume steps cover it.
`python3 -I lab/round/steps.py check lab/round/agents/<agent>` checks a
table; `start` and `resume` refuse one that fails.

| Kind | `text` | What the step does |
| --- | --- | --- |
| `agent` | the command line, optionally after `in:<dir>` (a directory below the home, `in:.` for the home; default `lab/proj`) | Stops the open session, if any, and starts the command in a new 120 by 40 terminal session `s<NN>`, inside an interactive login shell so `PATH` is a terminal's. |
| `prompt` | the prompt | Types the text, presses Enter and waits. |
| `type` | text | Types the text without Enter and waits. |
| `keys` | key names joined by commas: `Enter`, `Escape`, `Tab`, `Backspace`, `Space`, the arrows, `Home`, `End`, `PageUp`, `PageDown`, `Insert`, `Delete`, `F1` to `F12`, one letter or digit, each optionally after `Ctrl+`, `Shift+`, `Alt+` or `Meta+` | Presses the keys one second apart and waits. |
| `paste` | a file name under `agents/<agent>/pastes/` or `pastes/`, then optionally text to type after it | Copies the file into the VM, sends it as one bracketed paste (line feeds as carriage returns, as a terminal sends them), types the text, presses Enter and waits. |
| `command` | a shell command | Runs it in the VM outside the agent, in `~/lab/proj`, under a login shell (bash on Linux, zsh on macOS) or Windows PowerShell 5.1, and saves its output as `screens/NN-name.command.txt`. |
| `stop` | `-` | Saves the session's recording and stops it: Ctrl+C twice, then TERM (not on Windows), then a kill, each with a bounded wait. |
| `pause` | what the person must do; `{session}` becomes the open session's name | Snapshots the VM and exits 3, as described above. |
| `idle` | seconds | Waits that long. |
| `reboot` | `-` | Ends the session, reboots the VM from inside and waits for a new boot id over `ssh`. |
| `collect` | `-` | Runs the collector, the analyzer and the shape tool as `collect` does, into `collection/NN-name/`. |

`expect` says what a typing step waits for: `-` for the kind's default
(the agent's input marker for `agent`, `prompt` and `paste`, a settled
screen otherwise), `marker` for the input marker, `quiet` for a settled
screen, `exit` for the program's end, `none` for nothing, or a `grep -E`
pattern that must be on the settled screen, such as
`Allow once|Type your message` for a prompt that may raise a
confirmation dialog. A screen counts as settled once it has not changed
for `QUIET_SECONDS` while the agent's busy pattern is absent. A step
whose expectation is not met within `TURN_SECONDS` is recorded as
`missed` and the run goes on: a missed prompt is a finding about the
agent, repeated under the next number with a sharper prompt.

## Agent and VM definitions

| File | Holds |
| --- | --- |
| `agents/<agent>/agent.conf` | `AGENT_MARKER`, the `grep -E` pattern of the empty input box; `AGENT_BUSY`, the pattern shown while a turn runs; the default waits and the terminal size. |
| `agents/<agent>/install.sh`, `install.ps1` | The install command copied from the vendor's page, whose URL and fetch date the script prints as its `source:` line, then `bin:` and `version:` lines. The version is read from the package's own record where one exists, so the agent does not run before its first-launch step. |
| `agents/<agent>/roots` | The survey roots, from the Per-user storage, Credentials and Scratch directories sections of `research/<agent>.md`. |
| `agents/<agent>/steps.tsv` | The step table. |
| `agents/<agent>/pastes/`, `pastes/` | Paste files; the shared ones are a short traceback under 1 KB and release notes over 1 KB. |
| `vms/<VM>.conf` | `GUEST_OS` (`linux`, `macos` or `windows`), `GUEST_USER`, the clean snapshot's name (matched without case) and the tui-test asset with its sha256. |

A roots line is a path, `~` for the home and `<tmp>` for the per-user
temporary directory, globs allowed, optionally after `linux:`, `macos:`,
`unix:` (both) or `windows:`, where `%VARIABLE%` is expanded. A
`skip:<path>` line lists a directory but nothing below it (an installed
package tree). `keychain:<service>` prints whether the macOS keychain, or
on Windows Credential Manager, holds an item of that name, never its
value; on Linux it prints `not-checked`. `process:<name>` counts the
processes of that program, or of a script of that name under an
interpreter. Every listing also covers `~/lab/proj` and the top level of
the home, `~/.config`, `~/.local/share`, `~/.local/state`, `~/.local/bin`,
`~/.cache`, the macOS `Library` directories and the temporary
directories, as `top` lines, so a directory no root covers still shows.

## Where the evidence lands

| Path in the cell | Holds |
| --- | --- |
| `state`, `steps.log`, `round.log` | The cell's state (agent, VM, version, install source, tui-test version and sha256, the step to run next), one line per finished step with its status (`done`, `missed`, `failed`), and every command sent to the VM. |
| `steps.tsv`, `roots` | The cell's copy of the step table and roots; a fix during the round is made here, under the next step number. |
| `listings/NN-name.txt` | One listing: kind, size, mode, UTC modification time and path, tab-separated, never a file's contents. |
| `diffs/NN-name.txt` | The diff from the previous listing (`classify.py --redact`): added, removed and changed paths grouped by their first two components below the home, new top-level names, and changed credential-store items and process counts, with UUIDs and long hex runs printed as their class. |
| `screens/NN-name.txt` | The rendered screen before and after the step; `NN-name.command.txt` for a `command` step's output. |
| `casts/s<NN>.cast` | The asciicast of each terminal session. |
| `install.log`, `tools/` | The install script's output, the tui-test download, its self-test and the sample project's commit. |
| `collection/<label>/` | The collector's archive, manifest, summary, log and `.sha256`, its stdout, the sha256 check and the extracted archive. |
| `analysis/<label>/`, `shapes/<label>.txt` | The analyzer's `timeline.jsonl`, `sessions.jsonl` and `inventory.json`, and the shape tool's output with `--enums`. |
| `report.md` | Per step its status and diff counts, the missed and failed steps, and per collection the rows per status, the error rows, the `skipped_excluded` rows, the unflagged-key rows (collected, not `secret: true`, a file name holding key, token, secret, cred, cookie or auth), the analyzer's rows per numbered session and per row form, the forms an earlier collection lacked, and the `parser:` rows without the line they quote. |

tui-test keeps its own recordings and traces under `~/lab/tui-test` in
the VM (it keeps a recording only with tracing on). `~/lab` is the
pipeline's own directory and lies outside every agent's survey roots, so
neither shows in a diff.

The screens and recordings hold the agent's own output and the
collection holds its credentials: they stay in the cell directory, which
is deleted when the round is written up. A diff, the report and the
shape tool's output name files and keys only.

## Platforms

The Ubuntu path is the tested one. The macOS VM runs the same scripts
with the macOS tui-test build. The Windows VM gets `lib/survey.ps1`,
`lib/drive.ps1` and `lib/make-project.ps1`, the x86-64 tui-test build,
which runs under Windows 11's emulation, and the PowerShell collector
under Windows PowerShell 5.1, all over `ssh` with cmd.exe as the login
shell, so every guest command is a `.ps1` file or PowerShell reading
stdin; that path has not been run yet.

## Tests

```sh
python3 -m pytest lab/round/tests -q
```

The tests cover `classify.py`, the step table checker (including every
shipped agent's table) and the report.
