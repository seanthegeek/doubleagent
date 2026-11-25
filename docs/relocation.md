# Relocation warnings

An agent can keep its files somewhere other than its default folder when an
environment variable says so: `CLAUDE_CONFIG_DIR` moves Claude Code's
folder, `CODEX_HOME` moves Codex's, and `XDG_CONFIG_HOME` moves every
program that follows the XDG Base Directory Specification (the Linux
convention for where a program keeps its settings, data, state and cache).
A collection run looks for the names of such variables in the files that
set a user's or the system's environment, and warns when it finds one.
Where a file's syntax needs no shell to read, it also reads the value and
records it when it is a plain path: from `/etc/environment`,
`environment.d` files and launchd property lists, and on a live Windows
host from the registry. Read this page when a collection log or summary
reports a relocation hint, or when you expect an agent's files and the
collection has none.

## What the warning means

A relocation hint is one line of the log, and one entry in
`collection.json` `relocation.hints`, for each place a startup file names a
variable that can move an agent's files:

```text
relocation hint: /home/alice/.bashrc:41 names CLAUDE_CONFIG_DIR (claude-code)
```

It gives the file, the line number, the variable and the agent the
variable's catalog line belongs to. There is no line number for a value
read from a launchd property list as a whole or from the registry. Where
the run could read the value, the line ends with it; where it read an
assignment it could not use, with the reason
([Which values are read](#which-values-are-read)):

```text
relocation hint: /etc/environment:3 names CODEX_HOME (codex-cli) = /srv/codex
relocation hint: /home/alice/.config/environment.d/10-agents.conf:2 names CLAUDE_CONFIG_DIR (claude-code); value not resolved: references $XDG_CONFIG_HOME
```

The hints of each place come under a header line: `Relocation hints for
USER (HOME)`, `Relocation hints in project PATH`, `Relocation hints in
system files` or `Relocation hints in the registry`.

A hint says:

- The name appears on that line of that file, in the same case, with no
  letter, digit or underscore next to it. Shells name variables
  case-sensitively, so `codex_home` in a shell file is another variable and
  gives no hint. In a PowerShell file (a name ending in `.ps1`, in any case)
  the name matches in any case, as PowerShell variable names do, except
  just before a colon, where it is a drive or scope qualifier (`$env:PATH`
  names no `ENV`). In `/etc/environment` a name is also found where pam_env
  joins lines ending in a backslash: a name that only the joined text holds
  whole (`CODEX_\` on one line, `HOME=/opt/x` on the next) is a hint on the
  join's first line with the reason `continued`, under the rule of an older
  pam_env and of a newer one for a blank or comment line inside the join
  (see `/etc/environment` below). An `environment.d` file ends a name at the
  line end, as systemd does, so a name split there gives no hint.
- The agent's files may therefore be somewhere the catalog does not look,
  so the collection can be missing them without any other sign.

It does not say:

- That the agent is installed. A variable with a generic name (`LOG_DIR`,
  `LOG_PATH`, `DATABASE_PATH`, `CACHE_PATH`, `WORKSPACE_STORAGE_DIR`) is
  listed for the one agent known to read it, and another program may set
  it for its own files.
- That the variable is set. The line may be a comment, an `unset`, a
  branch of a conditional that did not run, or a mention of the name
  inside a longer command.
- What the value is, for a file whose values are not read. The run never
  evaluates a startup file and never writes out the line it matched; the
  hint's `value` in `collection.json` is empty. For the files
  [below](#which-values-are-read) it holds the path the line assigns, or is
  empty when the line assigns something else.
- That the agent's files moved. A variable set in a startup file may have
  been overridden by the process environment, a service definition or the
  registry, and an agent may ignore the variable on the platform you are
  looking at. A value is what one line assigns, not what the agent saw:
  when two files assign the same variable, both are recorded and neither
  is chosen.
- That the variable is absent when there is no hint. The search reads files
  and, on a live Windows host, the registry. A value set in a service or
  container definition, by a launcher or by another process that started
  the agent is not found, nor is one in the registry hive of a user whose
  hive was not loaded.

The file that holds each hint is collected, so you can read the line later.
Its manifest row has the agent `shared`, because it belongs to no single
agent, and is flagged `secret: true`, because startup files often hold
keys; `--no-secrets` leaves it out as `skipped_secret`, and the hint is
still recorded. A startup file the catalog already collects (a launchd
property list an agent installs, for example) keeps the catalog's row and
gets no second one.

A hint for a `shared` variable (the `XDG_*` names and the variables that
move the startup files themselves, such as `ZDOTDIR`) is a weaker signal
than one naming an agent's own variable. Many programs honour the `XDG_*`
variables and a user may set them for reasons that have nothing to do with
any agent, so check what the line says before you act on it. `ENV` is also
the Dockerfile instruction, so under `--env-scan-home` every Dockerfile in
the home gives an `ENV` hint. A hint on a variable that moves the startup
files themselves (`ZDOTDIR`, `ENV`, `BASH_ENV`) means the files it points
at were not searched, even when the hint carries the value: collect them by
hand and read them the same way.

## Which values are read

A value is read only from the sources below, whose syntax needs no shell,
and only for a line already reported as a hint. The run executes nothing a
file names and expands nothing from its own environment. The files are read
by their own rules on every platform, so a run on Windows over a Linux
image reads them as a run on Linux does.

| Source | Where the value comes from |
| --- | --- |
| `/etc/environment` | The text after `NAME=`, with one optional pair of double quotes removed. Nothing is expanded. A line starting with `#` is a comment. pam_env, the program that reads this file at login, also cuts a line at a `#`, joins a line ending in a backslash and blanks with the next line (an older pam_env skips blank and comment lines between them, a newer one ends the join there), skips an `export ` prefix and removes single quotes; the first two forms are refused (`holds a #`, `continued`, on every line of a join), since the file may assign a different value, and the last two are left as hints without a value. A carriage return ending the line stays in the value, as pam_env keeps it, so the value is refused (`holds a control character`). From a line holding a NUL byte on, nothing is a value (`after a NUL byte`), since pam_env 1.5 stops reading the file at such a line. |
| An `environment.d` file (`*.conf` in an `environment.d` directory) | The text after `NAME=`, read by the rules of `environment.d(5)`: white space around the name and the value is trimmed (a trailing space that an odd run of backslashes escapes stays, as systemd keeps it); a line starting with `#` or `;` is a comment; an odd run of backslashes at the end of an assignment line joins the next line to it, and the hint keeps the first line's number (an escaped backslash, `\\`, is a backslash and joins nothing, and neither does a backslash ending a comment or a line with no `=`); a quote left open joins the next line with its newline, as systemd reads a quoted value spanning lines, so such a value is refused (`holds a newline`) and the lines inside it assign nothing, and an escaped closing quote keeps the value open to the end of the file (`quoted`); one enclosing pair of single or double quotes is removed; `$$` is a dollar sign. `$HOME` and `${HOME}` become the user's home in a file that is under a collected user's home (`~/.config/environment.d`), and only there. Every other `$NAME`, `${NAME}`, `${NAME:-x}` and `${NAME:+x}` is left unresolved. From a line holding a NUL byte on, nothing is a value (`after a NUL byte`), since systemd reads the file only up to that byte. |
| A launchd property list (`*.plist` in a `LaunchAgents` or `LaunchDaemons` directory) | Read with `plutil`, the property list tool macOS ships, so the value is the one launchd reads: the key's value under the top-level `EnvironmentVariables` dictionary, the last one when a key repeats. A binary property list is read the same way when the name search finds a name in its bytes, which the format rarely allows; convert it with `plutil -convert xml1` to be sure. `plutil` reports no line, so the value is recorded on a hint of its own with `line` 0; the lines naming the variable are hints with an empty value. A system property list under `/Library/LaunchDaemons` gives the hint the list's `UserName` as its user when that is a plain account name (a letter or underscore, then letters, digits, `_`, `.` and `-`, at most 255 bytes), and an empty user otherwise. `plutil` is run only on macOS, whatever program of that name is on `PATH` elsewhere: on any other machine (a Linux or Windows run, over a macOS image too) the hint keeps an empty value with the reason `no plutil`. |
| The Windows registry | On a live Windows host only; see [The registry](#the-registry). |

A value from a file is kept only when it is a plain absolute path of at most 4,096 bytes (Linux's limit on the length of a path): one
starting with `/` that holds no `$`, `%`, backquote, quote (double or
single), control character (a tab, carriage return or newline) or `..`
segment, and does not start with a URI scheme such as `file:`. A trailing
`/` is dropped. In image mode the path is inside the image, so the root is
put in front of it (`-r /mnt/image` turns `/srv/cc` into
`/mnt/image/srv/cc`), in the log and in `collection.json`.

A value from the registry is kept only when it is a plain drive path: one
starting with a drive letter, a colon and a backslash or slash, that holds
no `%`, double quote, control character or `..` segment, and is at most 4,096 bytes long. A `$` or a
backquote is an ordinary path character on Windows and is kept. A trailing
separator is dropped, but a bare drive (`C:\` or `C:/`) is kept as `C:\`.

A line that assigns a value that does not pass keeps `value` empty, and its
log line ends with `; value not resolved: <reason>`. The reason names the
kind of text and never quotes it:

| Reason | The value |
| --- | --- |
| `longer than 4096 bytes` | Is longer than 4,096 bytes, in a file or the registry. The length of the text as assigned is tested before any other reason: for `/etc/environment` the text after `=`, quotes included; for an `environment.d` file the text after `=` with the blanks around it trimmed, quotes and joined lines included and before any expansion; for a property list the string `plutil` prints; for the registry the stored text before any `%NAME%` is replaced (the text after the replacements is tested too). No reason ever quotes text taken from such a value, and the hint never carries it. A value of exactly 4,096 bytes is judged by the other reasons. |
| `references $NAME` | Holds a reference to a variable, such as `$XDG_CONFIG_HOME`, `${HOME}` outside a user's own `environment.d` file, or `$HOME` in a file that is not `environment.d`. |
| `references %NAME%` | Holds a Windows reference that was not replaced. In a file, `NAME` is a word (letters, digits and underscores, not starting with a digit) between the value's first `%` and the next one. In the registry, `NAME` is any text there, a space included, but not empty. |
| `holds a $` | Its first `$` starts no name (a `$$`, a `$-`), whatever follows later in the value; in an `environment.d` file, which is expanded left to right first, a `$` that starts no reference (`$$`, `$-`, `${X` with no closing brace) once `$HOME` is expanded and no other reference follows it. |
| `holds a %` | Its first `%` starts no reference, whatever follows later in the value. In a file, the text after it is not a word closed by another `%` (a `%%`, a `%` with no `%` after it, or a name that is not a word). In the registry, it is a `%%` or a `%` with no `%` after it. |
| `quoted` | Holds a quote besides the one enclosing pair the file's syntax allows. |
| `holds a quote` | Holds a quote after the file's own unquoting, such as an escaped quote inside double quotes: a double or single quote in a file, a double quote in the registry. |
| `holds a backquote` | A file's value holds a backquote. |
| `holds a newline` | Holds a newline. |
| `holds a control character` | Holds a control character other than a newline (a tab, a carriage return, an escape); for a registry value, a newline too. |
| `holds a .. segment` | Holds a `..` path segment. |
| `holds a #` | Is in `/etc/environment` and holds a `#`, where pam_env cuts the line, so the file assigns a shorter value. |
| `continued` | Is in `/etc/environment` and ends in a backslash and blanks, which pam_env joins with the next line, or is a line joined that way, so the file may assign a longer value to the first line's variable and nothing to a later line's. Every line of a join is refused, and so is a name that only the joined text holds whole, on the join's first line. |
| `after a NUL byte` | Is an assignment in `/etc/environment` or an `environment.d` file that reaches or follows a line holding a NUL byte: pam_env 1.5 stops reading the file at such a line (a newer pam_env reads the line up to the byte and goes on) and systemd reads the file only up to the byte, so the file assigns less than the lines show, or nothing. |
| `uri` | Starts with a URI scheme, such as `file:///srv/cc`. |
| `relative` | Starts with a letter, digit, dot, underscore or hyphen and is not a URI, such as `opt/cc` or `./cc`. |
| `not an absolute path` | Is anything else that is not absolute, such as `~/cc`. |
| `not a drive path` | A registry value that does not start with a drive letter, a colon and a backslash or slash, such as `\\server\share`, `\Users\alice` or `opt/cc`. |
| `no plutil` | Is in a launchd property list read on a machine that is not macOS, or a Mac without `plutil`; every variable the list names gets this reason. |
| `plutil could not read the file` | Is in a launchd property list that `plutil -lint` rejects; every variable it names gets this reason. |
| `not a string` | A property list value that is not a string, or a registry value of a kind other than text. |
| `unreadable` | A registry value whose name is listed but whose value cannot be read. |

The `~/.pam_environment` and `/etc/security/pam_env.conf` files, shell
startup files, fish, Nushell, `mise`, `direnv`, `.env` files and PowerShell
profiles are not resolved: their hints give the file and line only. A hint
found by the `--env-scan-home` sweep in a file of one of the three kinds
above carries its value in the same way.

### The registry

On a live Windows host the relocation step also reads the registry, because
Windows keeps a user's and the machine's environment variables there rather
than in a file. It looks in the system environment key,
`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, and in
the `Environment` key of the `HKEY_USERS` hive of each collected user whose
hive is loaded and whose profile has a security identifier starting
`S-1-5-21-` (a local or domain account) or `S-1-12-` (a Microsoft Entra
account); `-u` limits which users these are. Every value whose name is a
relocation variable, in any case, gives a hint with line `0`, the key path
and value name as `source` (`HKU\S-1-5-21-...\Environment\CODEX_HOME`), and
the user's name (empty for the system key) as `user`.

The values are read in the order Windows reads them to build the
environment of a new logon, so each hint holds the value a new logon gives
the variable:

1. `SystemRoot`, from the Windows version key, and `SystemDrive`, from the
   system key or else `SystemRoot`'s drive.
2. The system key. A `REG_SZ` (plain text) value is taken as written, all of
   them first. Then each `REG_EXPAND_SZ` (expandable text) value, in the
   order the key lists them, has its `%NAME%` references replaced once from
   the names set so far.
3. For a user key, the profile's `USERPROFILE`, `APPDATA` and
   `LOCALAPPDATA`, taken from the user's profile path and the hive's
   `User Shell Folders` key.
4. The user key, as the system key in step 2. A user value replaces a
   system value of the same name, an empty value removes it, and a user
   `Path`, `LibPath` or `Os2LibPath` is added after the system value with a
   `;` between them (none when the system value ends with one).

So an expandable value sees `SystemRoot`, `SystemDrive`, its own key's
plain values and the expandable values listed before it, and, in a user
key, every system value and the profile's three names too. A replacement is
not searched for references again. A reference to an expandable value
listed later, to a user value from the system key, or to a name the
program does not know (such as `HOMEDRIVE`, `HOMEPATH` or
`ALLUSERSPROFILE`) stays in the text, and the value is not resolved
(`references %NAME%`). Limits keep a key built to exhaust memory from
doing so. A replacement that would make one value longer than 32,767 bytes
is not made, and that value is kept as written. Reading one key copies at
most 1 MiB of replacement text in all, counting text copied into a value
the 32,767-byte limit then refuses: the replacement that would pass it is
not made, that value and every later expandable value of the key are kept
as written, and the log says
`registry: KEY: more than 1 MiB of replaced text; the values after that are kept as written`,
with `KEY` the key path as a hint's `source` starts
(`HKU\S-1-5-21-...\Environment`), before that key's hints. A value kept as
written that still holds a reference is not resolved, and neither is a user
value built on a system value the system key kept as written; when the
1 MiB limit kept it, the log line names the system key, not the user's. A
user `Path`, `LibPath` or `Os2LibPath` that the join would make longer than
32,767 bytes is not added, and the variable keeps the system value. That
limit is the program's own: Windows joins the two whatever their length, so
a later reference to `%Path%` sees only the system value here where Windows
would see both. Names are compared ignoring the case of ASCII letters only;
Windows also ignores the case of other letters, so a reference differing
from a name by a non-ASCII letter's case stays unresolved here. The
environment of the process running the command is never consulted, so the
values describe the user's registry settings, not the account that ran the
collection.

The run never loads a hive: loading one with `reg load` or a profile loader
writes to the user's `NTUSER.DAT`, which an evidence collection must not
do. A user whose hive is not loaded is not read; the log says
`registry: hive of USER not loaded` and `collection.json` lists the user
under `relocation.registry.users_not_loaded`. On a default system only
Administrators can open another user's loaded hive: a hive that is loaded
but cannot be opened is logged as
`registry: hive of USER not readable: <message>` and listed under
`users_denied`, and an unreadable system key is logged as
`registry: system environment not readable: <message>`. Users read are in
`users_read`. An image-mode run and a run on Linux or macOS read no
registry, and `relocation.registry.reason` says `image mode` or
`not Windows` ([collection.json](collection-output.md#collectionjson)).

## Read the assignment in the collected file

Use this for a hint whose value is empty, such as one in a shell script, or
whose `value not resolved` reason you want to follow up. Extract the file
from the archive. Its path there is `fs/<original path>`, or the physical
path under its base for a file reached through a symlinked directory
([Archive layout](collection-output.md#archive-layout)), and the log line
names the original path and the line number. Look for the form that
matches the file's shell:

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
| `<key>NAME</key>` with the value in the next `<string>` | launchd property lists whose value did not resolve, and every property list read where `plutil` is missing |
| `[env]` table with `NAME = "value"` | `mise.toml` and the other `mise` configuration files |

Then check three things before you believe the value:

- A conditional. An assignment inside an `if`, a `case` or a test on the
  host name, the terminal or the operating system applies only when the
  condition holds. Read the lines above it.
- A sourced script. A line such as `. ~/.config/agent-env.sh` or
  `source ~/work/env` sets variables from another file. If the hint is in a
  file that names such a script, collect the script too (it may be in a
  directory the catalog does not reach) and read it the same way.
- A later line. A second assignment or an `unset` further down wins over
  the first.

## Confirm the live value on a running host

When the host is still up, check what a new login of the user sees, through
the EDR remote shell, as the user who ran the agent (or as an administrator
acting for that user). These read only, but a login shell runs the user's
startup files, which are the files you want to be careful with; prefer
reading the collected file when it answers the question.

| Platform and shell | Command |
| --- | --- |
| Linux or macOS, bash or zsh | `sudo -u USER -i printenv NAME` |
| Linux or macOS, fish | `sudo -u USER fish -l -c 'printenv NAME'` |
| Linux, the user's systemd session | `sudo -u USER XDG_RUNTIME_DIR=/run/user/UID systemctl --user show-environment` |
| macOS, the user's login session | `sudo launchctl asuser UID launchctl getenv NAME` |
| Windows, PowerShell | `[Environment]::GetEnvironmentVariable('NAME', 'User')`, then `'Machine'` |
| Windows, a hive listed under `users_not_loaded` | `reg query "HKU\<SID>\Environment" /v NAME` once the hive is loaded; a collection never loads one |
| Windows, a value the user's logon sets | `reg query "HKU\<SID>\Volatile Environment" /v NAME` (a collection does not read this key) |

Use these for a value a shell computes, for a variable set under a
condition, and for a user whose hive was not read. Replace `USER`, `UID`,
`SID` and `NAME`. An empty result means the variable is not set in that
place, not that the agent never saw it. The environment of a process that
is already running can differ from all of these; reading it is a task for
the EDR's process view.

## Collect the moved store

The catalog tries every entry against every home directory, so a store that
a variable moved to another place is not collected. `doubleagent collect`
has no option that takes a store's path yet
([issue 456](https://github.com/seanthegeek/doubleagent/issues/456)); a
resolved `value` is the path to use. Until then, make a second collection
with a root in which the store sits where the agent's default folder would
be:

1. Take the agent's default folder from the `Replaces` column of the
   `Relocation variables` table in the agent's research document, such as
   `.claude` for `CLAUDE_CONFIG_DIR`; where the table gives one path per
   platform, take your platform's. `--list` prints the first of them as the
   third field of the variable's line. A `.` there means the value stands
   in for the home itself (`GEMINI_CLI_HOME` holds a `.gemini` directory):
   copy the value's contents into `home/USER`.
2. Make an empty scratch directory outside the evidence and under it the
   path `home/USER/<default folder>`, where `USER` is the user who set the
   variable. On Windows, use `Users\USER\<default folder>`.
3. Copy the moved store there without changing it: `cp -a` on Linux and
   macOS, `robocopy /E /COPY:DAT /R:0 /XJ` on Windows, from the mounted
   image or from the live host. Both keep the modification times, which the
   second manifest then records from the copy; note the original path,
   owner and change times by hand, since the copy's manifest shows the
   scratch path and the copying account.
4. Run a collection with that scratch directory as the root, into a second
   output directory or the same one (every file name carries the host name
   and the time of the run, so the archives stay beside the first):

   ```sh
   ./doubleagent-linux-amd64 collect -r SCRATCH -u USER -o OUTPUT -q
   ```

   ```powershell
   .\doubleagent-windows-amd64.exe collect -r SCRATCH -u USER -o OUTPUT -q
   ```

The second archive's manifest paths begin with the scratch root, and its
`hostname` is the machine you ran it on. Keep a note of which variable,
which host and which original path the scratch directory stands for, since
the archive does not record it.

A store that holds project files rather than an agent's home (a directory
that has the agent's project-level files inside it, such as a `.claude`
folder, `AGENTS.md` or `.mcp.json`) needs no scratch root: give it as a
project with `-p DIR`, which collects the project-level files of the catalog
from it. A path inside the evidence is given with its mount point
([Options](collect-options.md#option-reference)).

## Variables the warning covers

`--list` prints every variable the search looks for, with the agent its
line belongs to and the default folder it replaces, after the Docker volume
names. Each agent's research document, listed in the
[research index](../research/README.md), explains its variables in its
`Relocation variables` subsection: what each moves, on which platforms, and
where in the agent's source or documentation that is said. Every agent
document that reads a configuration or cache folder through an `XDG_*`
variable also lists those variables there, as `shared`. The `shared`
variables are `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`,
`XDG_CACHE_HOME`, and the variables that move the startup files themselves
(`ZDOTDIR`, `ENV`, `BASH_ENV`, `ZSH_CUSTOM`, `BASH_IT_CUSTOM`, `OMF_CONFIG`),
listed in [Relocation variables in `research/shared.md`](../research/shared.md#relocation-variables).

The files searched are the last table of `--list`, `startup files searched
for them`: shell startup files, `/etc/environment`, `environment.d` files,
launchd property lists, PowerShell profiles, and in each project `.envrc`,
`.env` and the `mise` configuration, among others; each is named with its
source in
[Startup and environment files in `research/shared.md`](../research/shared.md#startup-and-environment-files).
A startup file that is itself a symlink is not read, nor one reached
through a symlinked directory that leads out of its base. A file that
starts with a UTF-16 or UTF-32 byte-order mark (as Windows PowerShell's
editor saves a profile) is decoded before it is searched.

## Search the whole home

A variable can also be set in a helper script those files source, or in a
tool-version file. `--env-scan-home` adds a sweep of every regular file in
each home and project that the catalog did not match, an exclusion did not
prune (ignoring ASCII case; a `volume|` exclusion applies only in a Docker
volume, never here) and the startup file search did not already read (a project inside a
home is swept once, as a project), and records a hint for each file that
names a variable, so `files_searched` counts each file once. The files the
catalog matched are in the archive already and are not searched.

Some places are not swept, and the log says so for each: a project that is
or holds a home directory (`/`, a working directory of `/home`), since that
would read the whole system (`sweep skipped: project PATH holds a home
directory`), and a home outside the usual home parents `/home`, `/Users`,
`/root`, `/var/root`, `/usr/home` and `/export/home`, such as a daemon
account's home like `/var/db/diagnostics` (`sweep skipped: home PATH lies
outside the user home roots`). The run's own log, manifest and staging
directory, which name every variable found, are never read, even when the
output directory is inside a home. Symlinks and other links are never
followed, and a file whose name holds a line break is not searched.

The sweep reads many files, so it adds time on a large home. A file bigger
than `--env-scan-max-size MB` (whole MiB, default `1`, `0` for no limit) is
not read and is counted in `relocation.files_skipped_size` of
`collection.json`. Binary files are searched like any other, as bytes, so
a name after a NUL byte is found. The sweep itself only reads; the files it
finds hints in are collected under the usual `--max-file-size` limit.

## Claude Code config homes

A second Claude Code config home named `~/.claude-<name>` (such as
`~/.claude-work`) is collected as `claude-code`, but only by the
Claude-specific names inside it. One outside the home directory, or not
named `.claude-<name>`, is not found; collect it by hand as above.

Claude Code reads its whole config home from `CLAUDE_CONFIG_DIR` when that
variable is set, and a second account in such a directory is common. The
names collected there:

| Name | Flagged `secret: true` |
| --- | --- |
| `projects/` | no |
| `file-history/` | no |
| `history.jsonl` | no |
| `.claude.json*` | `.claude.json` and `.claude.json.backup*` |
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
