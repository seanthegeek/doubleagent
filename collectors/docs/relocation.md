# Relocation warnings

An agent can keep its files somewhere other than its default folder when an
environment variable says so: `CLAUDE_CONFIG_DIR` moves Claude Code's
folder, `CODEX_HOME` moves Codex's, `XDG_CONFIG_HOME` moves every
program that follows the XDG Base Directory Specification (the Linux
convention for where a program keeps its settings, data, state and cache).
The collectors look for the names of such variables in the files that set
a user's or the system's environment and warn when they find one. Read this
page when a collection log or the summary reports a relocation hint, or
when you expect an agent's files and the collection has none.

## What the warning means

A relocation hint is one line of the log, and one entry in
`collection.json`, for each place a startup file names a variable
that can move an agent's files:

```text
relocation hint: /home/alice/.bashrc:41 names CLAUDE_CONFIG_DIR (claude-code)
```

It gives the file, the line number, the variable and the agent. It says:

- The name appears on that line of that file, bounded by characters that
  cannot be part of a name, in the same case. Shells name variables
  case-sensitively, so `codex_home` in a shell file is another variable and
  gives no hint; in a PowerShell file (a name ending in `.ps1`, in any case)
  the name matches in any case, as PowerShell variable names do.
- The agent's files may therefore be somewhere the catalog does not look,
  so the collection can be missing them silently.

It does not say:

- That the agent is installed. A variable with a generic name (`LOG_DIR`,
  `LOG_PATH`, `DATABASE_PATH`, `CACHE_PATH`, `WORKSPACE_STORAGE_DIR`) is
  listed for the one agent known to read it, and another program may set
  it for its own files.
- That the variable is set. The line may be a comment, an `unset`, a
  branch of a conditional that did not run, or a mention of the name
  inside a longer command.
- What the value is. The collectors never evaluate a startup file and
  never print the line they matched; the `value` field of each hint in
  `collection.json` is empty.
- That the agent's files moved. A variable set in a startup file may have
  been overridden by the process environment, a service definition or
  the registry, and an agent may ignore the variable on the platform you
  are looking at.
- That the variable is absent when there is no hint. The search reads files
  only. A value set in the Windows registry, in a service or container
  definition, by a launcher or by another process that started the agent
  is not found.

The file that holds each hint is collected, so you can read the line
later. It is attributed to the agent `shared` because it belongs to no
single agent, and it is flagged `secret: true` because startup files
often hold keys; `--no-secrets` leaves it out as `skipped_secret`, and
the hint is still recorded. A startup file the catalog already collects
(a launchd property list an agent installs, for example) keeps the
catalog's row and gets no second one.

A hint for a `shared` variable (the `XDG_*` names and the variables
that move the startup files themselves, such as `ZDOTDIR`) is a weaker
signal than one naming an agent's own variable. Many programs honour the
`XDG_*` variables and a user may set them for reasons that have nothing to
do with any agent, so check what the line says before you act on it. `ENV`
is also the Dockerfile instruction, so under `--env-scan-home` every
Dockerfile in the home gives an `ENV` hint. A hint on a variable that moves
the startup files themselves (`ZDOTDIR`, `ENV`, `BASH_ENV`) means the files
it points at were not searched, since no value is read: collect them by
hand and read them the same way.

## Read the assignment in the collected file

Extract the file from the archive. Its path in the archive is
`fs/<original path>` ([Archive layout in output.md](output.md#archive-layout)),
and the log line names the original path and the line number. Look for
the form that matches the file's shell:

| Syntax | Used by |
| --- | --- |
| `export NAME=value`, or `NAME=value` then `export NAME` | sh, bash, zsh, ksh, dash |
| `NAME=value` alone | `/etc/environment`, `environment.d` files and `.env` |
| `NAME DEFAULT=value` | `pam_env.conf` and `.pam_environment` |
| `set -gx NAME value`, or `SETUVAR --export NAME:value` in `fish_variables` | fish |
| `setenv NAME value` | csh and tcsh |
| `$env.NAME = "value"` | Nushell |
| `$NAME = "value"` | xonsh |
| `set E:NAME = value` | Elvish |
| `$env:NAME = "value"` | PowerShell profiles |
| `<key>NAME</key>` with the value in the next `<string>` | launchd property lists |
| `[env]` table with `NAME = "value"` | `mise.toml` and the other `mise` configuration files |

Then check three things before you believe the value:

- A conditional. An assignment inside an `if`, a `case` or a test on the
  host name, the terminal or the operating system applies only when the
  condition holds. Read the lines above it.
- A sourced script. A line such as `. ~/.config/agent-env.sh` or
  `source ~/work/env` sets variables from another file. If the hint is in
  a file that names such a script, collect the script too (it may be in a
  directory the catalog does not reach) and read it the same way.
- A later line. A second assignment or an `unset` further down wins over
  the first.

## Confirm the live value on a running host

When the host is still up, check what a new login of the user sees, through
the EDR remote shell, as the user who ran the agent (or as an
administrator acting for that user). These read only, but a login shell
runs the user's startup files, which are the files you want to be careful
with; prefer reading the collected file when it answers the question.

| Platform and shell | Command |
| --- | --- |
| Linux or macOS, bash or zsh | `sudo -u USER -i printenv NAME` |
| Linux or macOS, fish | `sudo -u USER fish -l -c 'printenv NAME'` |
| Linux, the user's systemd session | `sudo -u USER XDG_RUNTIME_DIR=/run/user/UID systemctl --user show-environment` |
| macOS, the user's login session | `sudo launchctl asuser UID launchctl getenv NAME` |
| Windows, PowerShell | `[Environment]::GetEnvironmentVariable('NAME', 'User')`, then `'Machine'` |
| Windows, another user's registry hive | `reg query "HKU\<SID>\Environment" /v NAME` (the hive must be loaded) |

Replace `USER`, `UID`, `SID` and `NAME`. An empty result means the variable
is not set in that place, not that the agent never saw it. The environment
of a process that is already running can differ from all of these; reading
it is a task for the EDR (its process view) and is out of scope for the
collectors.

## Collect the moved store

The catalog tries every entry against every home directory, so a store that
the variable moved to another place is not collected. The collectors have no
option that takes a store path yet; until
[issue 456](https://github.com/seanthegeek/doubleagent/issues/456) lands,
make a second collection by giving the collector a root in which the store
sits where the agent's default folder would be:

1. Take the agent's default folder from the `Replaces` column of the
   `Relocation variables` table in the agent's research document, such as
   `.claude` for `CLAUDE_CONFIG_DIR`; where the table gives one path per
   platform, take your platform's. `--list` prints the first of them as
   the third field of the variable's line.
2. Make an empty scratch directory outside the evidence and under it the
   path `home/USER/<default folder>`, where `USER` is the user who set the
   variable. On Windows, use `Users\USER\<default folder>`.
3. Copy the moved store there without changing it: `cp -a` on Linux and
   macOS, `robocopy /E /COPY:DAT /R:0 /XJ` on Windows, from the mounted
   image or from the live host. Both keep the modification times, which
   the second manifest then records from the copy; note the original
   path, owner and change times by hand, since the copy's manifest shows
   the scratch path and the copying account.
4. Run the collector with that scratch directory as the root, and a
   second output directory or the same one (every file name carries the
   host name and the time of the run, so the archives stay beside the first):

   ```sh
   collectors/collect-agent-artifacts.sh -r SCRATCH -u USER -o OUTPUT -q
   ```

   ```powershell
   .\Collect-AgentArtifacts.ps1 -Root SCRATCH -Users USER -OutputDir OUTPUT -Quiet
   ```

The second archive's manifest paths begin with the scratch root, and its
`hostname` is the machine you ran it on. Keep a note of which variable, which
host and which original path the scratch directory stands for, since the
archive does not record it.

A store that holds project files rather than an agent's home (a directory
that has the agent's project-level files inside it, such as a `.claude`
folder, `AGENTS.md` or `.mcp.json`) needs no scratch root: give it to the
collector as a project with `-p DIR` (`-Project DIR`), which collects the
project-level files of the catalog from it. A path inside the evidence
is given with its mount point, as [options.md](options.md) describes.

## Variables the warning covers

`--list` prints every variable the search looks for, with the agent its
line belongs to and the default folder it replaces, after the Docker
volume names. Each agent's research document, listed in the
[research index](../../research/README.md), explains its variables in its
`Relocation variables` subsection: what each moves, on which platforms,
and where in the agent's source or documentation that is said. Every agent
document that reads a configuration or cache folder through an `XDG_*`
variable also lists those variables there, as `shared`. The `shared`
variables are `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`,
`XDG_CACHE_HOME`, and the variables that move the startup files themselves
(`ZDOTDIR`, `ENV`, `BASH_ENV`, `ZSH_CUSTOM`, `BASH_IT_CUSTOM`, `OMF_CONFIG`),
listed in [Relocation variables in `research/shared.md`](../../research/shared.md#relocation-variables).

## Search the whole home

The search reads only the files that set an environment: the shell
startup files, `/etc/environment`, launchd property lists, `.envrc`,
`.env` and the `mise` configuration in each project, and the like. A
variable can also be set in a helper script those files source, or in a
tool-version file. `--env-scan-home` (PowerShell: `-EnvScanHome`) adds a
sweep of every regular file in each home and project that the catalog did
not match, an exclusion did not prune and the startup file search did not
already read (a project inside a home is swept once, as a project), and
records a hint for each file that names a variable, so `files_searched`
counts each file once. The files the catalog matched are in the archive
already and are not searched; a project that is or holds a home directory
(`/`, a user's cwd of `/home`) is not swept, since that would read the
whole system, and the log says so; the run's own log, manifest and
staging directory, which name every variable found, are never read, even
when the output directory is the home. Symlinks are never followed.

The sweep reads many files, so it adds time on a large home. A file bigger
than `--env-scan-max-size MB` (PowerShell `-EnvScanMaxSizeMB`; whole MiB,
default `1`, `0` for no limit) is not read and is counted in
`files_skipped_size` of `collection.json`. Binary files are searched like any
other; the sh collector reads a file holding a NUL byte as text only where
`grep -a` does so, which `collection.json` records as `capabilities.grep_text`.
The sweep itself only reads; the files it finds hints in are
collected under the usual `--max-file-size` cap.

## Claude Code config homes

A second Claude Code config home named `~/.claude-<name>` (such as
`~/.claude-work`) is collected as `claude-code`, but only by the
Claude-specific names inside it. One outside the home directory, or not
named `.claude-<name>`, is not found; collect it by hand.

Claude Code reads its whole config home from `CLAUDE_CONFIG_DIR` when that
variable is set, and a second account in such a directory is common. The
names collected there:

| Name | Flagged `secret: true` |
| --- | --- |
| `projects/` | no |
| `file-history/` | no |
| `history.jsonl` | no |
| `.claude.json*` | `.claude.json` only |
| `.credentials.json` | yes |
| `hfi-auth.json` | yes |
| `.session_ingress_token` | yes |
| `remote/.oauth_token` | yes |
| `remote/.api_key` | yes |
| `sessions/*.*.key` (the `<pid>.<sha256>.key` session keys) | yes |
| `daemon/control.key`, `daemon/pipe.key`, `daemon/roster.json` | yes |
| `backups/.claude.json.*` | yes |
| `ide/*.lock` | yes |
| `bridge-spawn/*/mcp.json` | yes |

The rest of a relocated home (`settings.json`, `todos/`, `plans/`,
`shell-snapshots/` and the other subdirectories) is not collected, because
a bare `.claude-*` entry would also take other tools' directories such as
`~/.claude-code-router` and `~/.claude-mem`.

## What comes next

[Issue 455](https://github.com/seanthegeek/doubleagent/issues/455) will
resolve plain values (the registry, `/etc/environment`, launchd property
lists) so the hint gives the path, and
[issue 456](https://github.com/seanthegeek/doubleagent/issues/456) will
collect and parse the store at that path in the same run. When they land,
the second collection above and most of the live checks are replaced
by the collector's output, and this page shrinks with them.
