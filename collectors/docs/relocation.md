# Relocation warnings

An agent can keep its files somewhere other than its default folder when an
environment variable says so: `CLAUDE_CONFIG_DIR` moves Claude Code's
folder, `CODEX_HOME` moves Codex's, `XDG_CONFIG_HOME` moves every
program that follows the XDG Base Directory Specification (the Linux
convention for where a program keeps its settings, data, state and cache).
The collectors look for the names of such variables in the files that set
a user's or the system's environment and warn when they find one. Where the
file's syntax needs no shell to read, they also read the value and record it
when it is a plain path. Read this page when a collection log or the summary
reports a relocation hint, or when you expect an agent's files and the
collection has none.

## What the warning means

A relocation hint is one line of the log, and one entry in
`collection.json`, for each place a startup file names a variable
that can move an agent's files:

```text
relocation hint: /home/alice/.bashrc:41 names CLAUDE_CONFIG_DIR (claude-code)
```

It gives the file, the line number (none for a value `plutil` read from a launchd plist, or from the registry), the variable and the agent. Where the
collector could read the value, the line ends with it, and where it read an
assignment it could not use, with the reason ([What the collector
resolves](#what-the-collector-resolves)):

```text
relocation hint: /etc/environment:3 names CODEX_HOME (codex-cli) = /srv/codex
relocation hint: /home/alice/.config/environment.d/10-agents.conf:2 names CLAUDE_CONFIG_DIR (claude-code); value not resolved: references $XDG_CONFIG_HOME
```

A hint says:

- The name appears on that line of that file, bounded by characters that
  cannot be part of a name, in the same case. Shells name variables
  case-sensitively, so `codex_home` in a shell file is another variable and
  gives no hint; in a PowerShell file (a name ending in `.ps1`, in any case)
  the name matches in any case, as PowerShell variable names do, except
  before a colon, where it is a drive or scope qualifier (`$env:PATH` names
  no `ENV`).
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
- What the value is, for a file the collector does not resolve. The
  collectors never evaluate a startup file and never print the line they
  matched; the `value` field of such a hint in `collection.json` is empty.
  For the files [below](#what-the-collector-resolves) it holds the path the
  line assigns, or is empty when the line assigns something else.
- That the agent's files moved. A variable set in a startup file may have
  been overridden by the process environment, a service definition or
  the registry, and an agent may ignore the variable on the platform you
  are looking at. A resolved value is what one line assigns, not what the
  agent saw: when two files assign the same variable, the collector records
  both and does not choose between them.
- That the variable is absent when there is no hint. The search reads files
  and, on a live Windows host, the registry. A value set in a service or
  container definition, by a launcher or by another process that started the
  agent is not found, nor is one in the registry hive of a user whose hive was
  not loaded.

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
it points at were not searched, even when the hint carries the value:
collect them by hand and read them the same way.

## What the collector resolves

The collector reads a value only in the places whose syntax needs no shell,
and only for a line it already reported as a hint. It runs no program the
file names and expands nothing from its own environment.

| File | Where the value comes from |
| --- | --- |
| `/etc/environment` | The text after `NAME=`, with one optional pair of double quotes removed. Nothing is expanded. A line starting with `#` is a comment. pam_env also cuts a line at a `#`, joins a line ending in a backslash and blanks with the next line that is not blank or a comment, skips an `export ` prefix and removes single quotes; the collector refuses the first two forms (`holds a #`, `continued`, on both lines of a join), since the file assigns a different value, and leaves the last two as hints without a value. A carriage return ends a line here, where pam_env would keep it in the value. |
| An `environment.d` file (`*.conf` in an `environment.d` directory) | The text after `NAME=`, read by the rules of `environment.d(5)`: white space around the name and the value is trimmed; a line starting with `#` or `;` is a comment; a backslash at the end of a line joins the next line to it, and the hint keeps the first line's number; one enclosing pair of single or double quotes is removed; `$$` is a dollar sign. `$HOME` and `${HOME}` become the user's home in a file that is under a collected user's home (`~/.config/environment.d`), and only there. Every other `$NAME`, `${NAME}`, `${NAME:-x}` and `${NAME:+x}` is left unresolved. |
| A launchd property list (`*.plist` in a `LaunchAgents` or `LaunchDaemons` directory) | Read with `plutil`, the property list tool macOS ships, so the value is the one launchd reads: the key's effective value under the top-level `EnvironmentVariables` dictionary, the last value when a key repeats (a binary plist is read the same way when the name search finds a name in its bytes, which the format rarely allows; convert it with `plutil -convert xml1` to be sure). plutil reports no line, so the value is recorded on a hint of its own with `line` 0, as a registry value is; the lines naming the variable are hints with an empty value. A system plist under `/Library/LaunchDaemons` gives the hint the plist's `UserName` as its user when it is a plain account name, and an empty user otherwise. Without `plutil` (Linux, Windows, an image examined off macOS) the hint keeps an empty value with the reason `no plutil`. |
| The Windows registry (PowerShell collector, live Windows host) | See [The registry](#the-registry). |

A value is kept only when it passes one test, whichever file it came from:
it is an absolute path (starting with `/`, or with a drive letter and a
backslash or slash for a registry value) that holds no `$`, `%`, backquote, quote,
control character (a tab, carriage return or newline) or `..` segment, and
does not start with a URI scheme such as `file:`. A trailing `/` is dropped.
In image mode the path is inside the image, so the collector puts the root
in front of it (`-r /mnt/image` turns `/srv/cc` into `/mnt/image/srv/cc`; the
PowerShell collector on Windows writes the image path with backslashes), in
the log, in `collection.json` and in the value list the collector keeps for
[issue 456](https://github.com/seanthegeek/doubleagent/issues/456).

A line that assigns but does not pass keeps `value` empty, and the log ends
with `; value not resolved: <reason>`. The reason names the kind of text and
never quotes it. The collectors use these reasons:

| Reason | The value |
| --- | --- |
| `references $NAME` | Holds a reference to a variable, such as `$XDG_CONFIG_HOME`, `${HOME}` outside a user's own `environment.d` file, or `$HOME` in a file that is not `environment.d`. |
| `references %NAME%` | Holds a Windows reference that was not replaced. |
| `holds a $` or `holds a %` | Its first `$` or `%` starts no name (a `$$`, a `$-`, a `%%`), whatever follows later in the value; in an `environment.d` file, which is expanded left to right first, a `$` that starts no reference (`$$`, `$-`, `${X` with no closing brace) once `$HOME` is expanded and no other reference follows it. |
| `quoted` | Holds a quote besides the one enclosing pair the file's syntax allows. |
| `holds a quote` | Holds a quote after the file's own unquoting, such as an escaped quote inside double quotes. |
| `holds a backquote` | Holds a backquote. |
| `holds a newline` or `holds a control character` | Holds a newline, or any other control character (a tab, a carriage return, an escape). |
| `holds a .. segment` | Holds a `..` path segment. |
| `holds a #` | Is in `/etc/environment` and holds a `#`, where pam_env cuts the line, so the file assigns a shorter value. |
| `continued` | Is in `/etc/environment` and ends in a backslash and blanks, which pam_env joins with the next line that is not blank or a comment, or is that next line, so the file assigns a longer value to the first line's variable and nothing to the second's. |
| `holds an invalid byte` | PowerShell collector only: holds a byte sequence that is not UTF-8, which .NET reads as the replacement character (U+FFFD), or that character itself; the sh collector keeps the bytes as they are. |
| `uri` | Starts with a URI scheme, such as `file:///srv/cc`. |
| `relative` | Starts with a letter, digit, dot, underscore or hyphen and is not a URI, such as `opt/cc` or `./cc`. |
| `not an absolute path` | Is anything else that is not absolute, such as `~/cc`. |
| `no plutil` | Is in a launchd plist on a host without `plutil` (not macOS); every variable the plist names gets this reason. |
| `plutil could not read the file` | Is in a launchd plist that `plutil -lint` rejects; every variable it names gets this reason. |
| `not a string` | Is in a plist, or in a registry value of a kind other than text, and is not a string. |
| `holds an XML entity` | Is a plist string that `plutil` had to encode with an entity (`&amp;`). |

The `~/.pam_environment` and `/etc/security/pam_env.conf` files, shell startup files,
fish, Nushell, `mise`, `direnv`, `.env` files and PowerShell profiles are
not resolved: their hints give the file and line, as before. A hint found by
the `--env-scan-home` sweep in a file of one of the three kinds above carries
its value in the same way.

### The registry

On a live Windows host the PowerShell collector also reads the registry. It
looks in the system environment key, `HKLM\SYSTEM\CurrentControlSet\Control\Session
Manager\Environment`, and in the `Environment` key of the `HKEY_USERS` hive
of each collected user whose hive is loaded. Every value whose name is a
relocation variable (in any case) gives a hint with line `0`, the key path
and value name as `source`, and the user's name (empty for the system key)
as `user`. The log groups them under `Relocation hints in the registry`.

A value of type `REG_SZ` is read as written. A `REG_EXPAND_SZ` value has its
`%NAME%` references replaced from, in this order:

1. The other text values of the same key.
2. For a user key, the profile: `USERPROFILE`, `HOMEDRIVE`, `HOMEPATH`,
   `APPDATA` and `LOCALAPPDATA`, taken from the user's profile path and the
   hive's `User Shell Folders` key.
3. The system key's text values, `SystemRoot` and `SystemDrive`.

A reference found nowhere stays in the text and the value is not resolved
(`references %NAME%`). The collector's own environment is never consulted, so
the values describe the user's registry settings, not the account that ran the
collector.

A user whose hive is not loaded is not read: the collector never loads a hive, since loading changes the user's
`NTUSER.DAT`. The log says `registry: hive of <user> not loaded` and
`collection.json` lists the user under `relocation.registry.users_not_loaded`.
A hive that is loaded but cannot be opened is logged as
`registry: hive of <user> not readable: <message>` and listed under
`users_denied`; an unreadable system key is logged as
`registry: system environment not readable: <message>`. Users read are in
`users_read`. [output.md](output.md#collectionjson) lists the
`registry` object, and [windows.md](windows.md#the-registry-step) says what
needs Administrator. The sh collector, an image-mode run and PowerShell
off Windows read no registry and record why in `registry.reason`.

## Read the assignment in the collected file

Use this for a hint whose value is empty, such as one in a shell script, or
whose `value not resolved` reason you want to follow up. Extract the file from
the archive. Its path in the archive is
`fs/<original path>`, or the physical path under its base for a file reached
through a symlinked directory ([Archive layout in output.md](output.md#archive-layout)),
and the log line names the original path and the line number (the path alone for a value read from a launchd plist as a whole). Look for
the form that matches the file's shell:

| Syntax | Used by |
| --- | --- |
| `export NAME=value`, or `NAME=value` then `export NAME` | sh, bash, zsh, ksh, dash |
| `NAME=value` alone | `/etc/environment` and `environment.d` files whose value did not resolve, and `.env` |
| `NAME DEFAULT=value` | `pam_env.conf` and `.pam_environment` |
| `set -gx NAME value`, or `SETUVAR --export NAME:value` in `fish_variables` | fish |
| `setenv NAME value` | csh and tcsh |
| `$env.NAME = "value"` | Nushell |
| `$NAME = "value"` | xonsh |
| `set E:NAME = value` | Elvish |
| `$env:NAME = "value"` | PowerShell profiles |
| `<key>NAME</key>` with the value in the next `<string>` | launchd property lists whose value did not resolve, and every plist on a host without `plutil` |
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
| Windows, a hive the collector listed under `users_not_loaded` | `reg query "HKU\<SID>\Environment" /v NAME` once the hive is loaded; the collector never loads one |
| Windows, a value the user's logon sets | `reg query "HKU\<SID>\Volatile Environment" /v NAME` (the collector does not read this key) |

Use these for a value a shell computes, for a variable set under a condition,
and for a user the collector listed as not read. Replace `USER`, `UID`, `SID`
and `NAME`. An empty result means the variable is not set in that place, not
that the agent never saw it. The environment
of a process that is already running can differ from all of these; reading
it is a task for the EDR (its process view) and is out of scope for the
collectors.

## Collect the moved store

The catalog tries every entry against every home directory, so a store that
the variable moved to another place is not collected. The collectors have no
option that takes a store path yet, and a resolved `value` is the path to
use as the store; until
[issue 456](https://github.com/seanthegeek/doubleagent/issues/456) lands,
make a second collection by giving the collector a root in which the store
sits where the agent's default folder would be:

1. Take the agent's default folder from the `Replaces` column of the
   `Relocation variables` table in the agent's research document, such as
   `.claude` for `CLAUDE_CONFIG_DIR`; where the table gives one path per
   platform, take your platform's. `--list` prints the first of them as
   the third field of the variable's line. A `.` there means the value
   stands in for the home itself (`GEMINI_CLI_HOME` holds a `.gemini`
   directory): copy the value's contents into `home/USER`.
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
whole system, nor is a home outside the usual home roots (`/home`,
`/Users`, `/root`, `/var/root`, `/usr/home`, `/export/home`: a daemon
account's home such as `/var/db/diagnostics`), and the log says so for
each; the run's own log, manifest and staging directory, which name every
variable found, are never read, even when the output directory is the
home. Symlinks are never followed, and a file whose name holds a line
break is not searched. The startup file search covers every home, these
included. The sh collector searches bytes, so a UTF-16 file (a Windows
PowerShell profile saved by the ISE) on a mounted Windows image gives it
no hint; the PowerShell collector reads such a file as text.

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

[Issue 456](https://github.com/seanthegeek/doubleagent/issues/456) will
collect and parse the store at the resolved path in the same run. When it
lands, the second collection above is replaced by the collector's output,
and this page shrinks with it.
