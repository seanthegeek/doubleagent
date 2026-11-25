# Deployment

During an incident a collection usually reaches a host through the remote
shell of the EDR (endpoint detection and response) product the responder
already has, not a terminal on the host: you upload the one `doubleagent`
executable, run it once, and retrieve the archive it wrote, or run it in
inventory mode across the fleet and keep what it printed. The consoles hold
a command's output until it ends and bound how long one command may run, so
a collection through one is planned around those limits. This page also
covers collecting from a mounted disk image, and what a run does to the
access times of the files it reads.

## Getting the program onto a host

Download the build for the host's operating system and processor from the
[releases page](https://github.com/seanthegeek/doubleagent/releases), with
`SHA256SUMS`, and check it on your own workstation first
([Getting the program](collect.md#getting-the-program)). Then upload the
executable with the console's file upload (in CrowdStrike Falcon RTR,
`put`), to a directory you can also retrieve files from.

Check the uploaded copy on the host against the same line of `SHA256SUMS`
before you run it:

```sh
sha256sum /var/tmp/ir/doubleagent-linux-amd64           # Linux
shasum -a 256 /var/tmp/ir/doubleagent-darwin-arm64      # macOS
chmod +x /var/tmp/ir/doubleagent-linux-amd64
```

```powershell
Get-FileHash C:\ir\doubleagent-windows-amd64.exe        # SHA-256 is the default
```

`Get-FileHash` prints the hash in capitals; compare it with the
`SHA256SUMS` line without regard to case. An upload usually loses the
executable bit on Linux and macOS, hence the `chmod`.

## Collection

An EDR product's remote shell is the usual way to reach a host:
CrowdStrike Falcon RTR (Real Time Response), SentinelOne RemoteOps (Remote
Operations, which runs a script on a host as a console task), Microsoft
Defender for Endpoint Live Response, or Palo Alto Networks Cortex XDR
(extended detection and response) Live Terminal. Where a console runs only
scripts, start the program from a short script that you upload beside it.

`doubleagent collect` asks nothing, reads nothing from standard input, and
writes everything under `-o`. Run it with `-o` pointing at the directory
you can retrieve from, then pull the archive (always `.tar.gz`) and its
`.sha256` file. Use `-q` to keep the console output to the final summary.

On Windows, files held open by a running program still copy, and paths
longer than 260 characters are collected without any host setting
([Locked files and long paths](windows.md#locked-files-and-long-paths)).

### Console output arrives at the end

A run writes its progress as it goes: one line per user, catalog match and
project, to standard error unless `-q` is given, and always to the log file
under `-o`, one line at a time. In a local terminal that progress is live.

The EDR consoles hold it back: they run a command whose result comes back
when it ends, so a long collection shows nothing until the summary. That is
the console's behaviour, not a hang. `-q` changes nothing here: it removes
the progress lines from standard error, but the console holds standard
output and standard error alike until the end.

The consoles also bound how long one command may run:

| Console | Command limit | Source |
| --- | --- | --- |
| CrowdStrike RTR | `runscript` takes `-Timeout=<seconds>`. The run-script command of the Falcon integration for Cortex XSOAR (Palo Alto Networks' security orchestration, automation and response platform) documents a maximum of 600 seconds (default 30). The RTR session itself times out after 10 minutes unless you interact with it. | [Cortex XSOAR's CrowdStrike Falcon integration reference](https://xsoar.pan.dev/docs/reference/integrations/crowdstrike-falcon) (the 600-second maximum); a CrowdStrike maintainer in [CrowdStrike/psfalcon discussion 249](https://github.com/CrowdStrike/psfalcon/discussions/249), who sets a 600-second command timeout and recommends launching a long script as a separate process and collecting its results afterwards; the [UAC documentation on Falcon RTR](https://tclahr.github.io/uac-docs/using_uac_with_cs_falcon_rtr/) (the session timeout, and a background run of UAC, the Unix-like Artifacts Collector, watched through its log) |
| Defender Live Response `RunScript` | Times out after 10 minutes. | [Run live response commands on a device](https://learn.microsoft.com/en-us/defender-endpoint/api/run-live-response) |
| SentinelOne RemoteOps script task | Set per task in the console; the script's output is read from the task's status in the console once the task has run. | A [Safety deployment guide for SentinelOne](https://docs.safetycli.com/safety-docs/deployment/deployment/sentinelone), which recommends a timeout of at least 1800 seconds for its own long-running script |

(Sources fetched 2026-10-15.)

### Long or watched collections

When a collection may run longer than the console's command limit, or you
want to watch it, start it detached from the console's command so the
command returns at once, then read the log from the same session. A
built-in option for this is tracked in
[issue 101](https://github.com/seanthegeek/doubleagent/issues/101).

On Linux and macOS (the paths are examples):

```sh
nohup /var/tmp/ir/doubleagent-linux-amd64 collect -o /var/tmp/ir -q >/dev/null 2>&1 &
```

On Windows, from PowerShell:

```powershell
Start-Process -FilePath C:\ir\doubleagent-windows-amd64.exe -ArgumentList 'collect -o C:\ir -q' -WindowStyle Hidden
```

The log is created in the output directory when the run starts, under the
name [Output files](collection-output.md#output-files) gives,
`<host>_<UTC time>_agent-artifacts.log`. Every line starts with a UTC time.
Read it with `cat` or `tail` (RTR's `cat` on Windows, or `Get-Content -Tail`
in PowerShell) as often as you like:

- One line per user with agent state, per catalog match (the agent name in
  brackets, then the matched path) and per project is written as the walk
  reaches it, so the lines show which home and agent the run is on. The log
  keeps no running count of files; the counts are in the summary at the
  end.
- `Archiving` marks the end of the walk; the archive is being written, as
  `<archive>.partial`, and is not complete yet.
- The last line, `<UTC time> done: <archive path> (<size> bytes, sha256
  <hex>)`, is written once the archive is complete, hashed and moved to
  its name ([Log](collection-output.md#log)). Pull the archive only after
  it appears. The archive has its own name only once the run has
  finished, so the archive under that name with its `.sha256` file beside
  it is the sign the run completed, whatever the log's last line: a run
  whose write of the `done:` line fails still finishes and exits `0`.

Then retrieve the archive and its `.sha256` file, and check them on your
workstation with `sha256sum -c <archive>.sha256` (`shasum -a 256 -c` on
macOS). With the output sent to `/dev/null` the stdout summary is not kept,
but the same counts are in `collection.json` beside the archive
([collection.json](collection-output.md#collectionjson)). A run that stops
before it creates the log (a usage error, an output directory that cannot
be created) leaves no log at all, and its message went to the discarded
standard error; run the same command in the foreground to see it.

A collection or inventory run stops early only when it is told to stop or
meets a fatal error. A console that goes away while a collection run is in
the foreground (a session that times out or detaches, a terminal that
hangs up, a pipe whose reader ends) does not stop it. On Linux and macOS the
program catches the `SIGHUP` and `SIGPIPE` signals such a loss sends
rather than being ended by them, and on Windows a write to a closed pipe
fails without a signal; either way the run drops what it cannot print,
writes the archive, removes the staging directory and exits `0`, and the
log records the whole run. A `--inventory` run, whose only output is
standard output, exits `2` instead when that output is a pipe whose reader
ended or a terminal that hung up; when standard output was closed before it started (`>&-` in a
shell), the program is given the null device in its place, so the
inventory is lost and the run exits `0`.

`INT` and `TERM` on Linux and macOS, and Ctrl+C, Ctrl+Break or closing the
console window on Windows, are requests to stop: the run logs
`interrupted`, removes the staging directory, the partial archive, its
`.sha256` file and `collection.json`, and exits `130`, leaving the log and
the manifest of what it did until then, if Windows leaves it the few
seconds it gives before it ends the process after a window is closed. A
run started in the background (`&`) by a shell that is not interactive,
such as the `nohup` line above sent as a script, starts with `INT` ignored
and keeps it so: stop such a run with `kill -TERM`. A
request that arrives once the archive's move to its name has begun, just
before the `done:` line, changes nothing: the run finishes as if it had
not come. At a logoff or shutdown on Windows, a program started from an
interactive session is ended without being told, so the run sees no
request; a run started by a service (an EDR agent, for example) may
receive the logoff or shutdown event, which is then a request to stop, as
a closed console window is, and the logoff event is sent when any user
logs off, without saying which
([HandlerRoutine callback function](https://learn.microsoft.com/en-us/windows/console/handlerroutine)).
A run ended that way, or by `SIGKILL`, or by `SIGQUIT` (which prints a
stack dump of the program and exits `2`), leaves its staging directory,
`.stage-<host>_<UTC time>_agent-artifacts`, in the output directory with
the copies made so far, and a log without a closing line, and, when it was
archiving, a `.partial` archive and perhaps a `.sha256` file; a run ended
after the archive was moved to its name but before the `done:` line leaves
the archive under its own name with its `.sha256` file, complete. A `.partial`
archive with a `.sha256` file beside it is complete: rename it to the name
the `.sha256` file gives and check it with `sha256sum -c`
(`shasum -a 256 -c` on macOS), as
[Output files](collection-output.md#output-files) says; the same holds when
a run exits `2` because it could not move the archive to its name. A
`.partial` archive without a `.sha256` file may be incomplete: delete it,
and the staging directory, by hand. On Linux and macOS only root and the account that ran
`collect` can read the staging directory.
`--list`, `--version` and `--help` write no files, and a pipe whose reader
ended stops them on Linux and macOS.

### Runtime

A run's time grows with the number and size of the files collected, since
each one is copied and hashed: long agent histories (years of transcripts,
large per-project histories, Docker volumes) and `--env-scan-home` on a
large home make it minutes rather than seconds. Plan for the console limits
above, and use the detached form when in doubt.

## Fleet inventory

For a fleet inventory, run `doubleagent collect --inventory -q` with no
`-o`: nothing is written on the host or retrieved from it, and the
command's standard output is the result. Save each host's standard output
as its own file (the EDR console's output export, or a copy of the
response) and pass the files, or a directory of them, to
`doubleagent fleet` on your workstation ([Fleet inventory](fleet.md)).
`-q` keeps standard error empty; the two never mix, but some consoles show
both together, and `fleet` skips the lines that are not inventory records.
The lines are described in [Inventory mode](collect-inventory.md).

## Disk images

To collect from a disk image, mount it on your workstation, read-only, and
give the mount point as the root:

```sh
./doubleagent-linux-amd64 collect -r /mnt/evidence -o /cases/host01
```

In image mode (`-r`):

- Every path is looked for under the root, and every path in the manifest,
  the log and `collection.json` keeps the root in front; archive members
  leave it off (`fs/home/alice/...`).
- Homes come from the image's own `etc/passwd`, then from its home parents
  (`home/*`, `Users/*`, `root`, `var/root`, `usr/home/*`, `export/home/*`),
  by the same rule on every workstation
  ([Home directory discovery](coverage.md#home-directory-discovery)); no
  program is run to list users, and on Windows no registry is read.
- A link in the image whose target leads out of the root (an absolute link
  such as `/home/alice/.config -> /srv/dotfiles` points, on your
  workstation, at your own files) is not followed in any step; the log
  names it once, and a catalog match that runs through it gets a `symlink`
  row for the link.
- `collection.json` `hostname`, and the output file names, are your
  workstation's name, not the imaged host's: note which host the archive
  stands for.
- On Linux and macOS `-r /` is the live system, not an image. On Windows
  every `-r` is image mode, a drive root included.

Every catalog line is tried against every home, so the image of any
platform can be collected from any other. A Windows image mounted on Linux
or macOS is collected through its `Users` directory, with the `AppData`
catalog lines; its rows then carry the numeric owners and modes the mount
reports, not Windows owners and attributes, and the registry's environment
values are not read. Every pattern ignores ASCII letter case, so a mount
that keeps case still matches a directory spelled otherwise than the
catalog (`appdata` for `AppData`)
([Case variants](coverage.md#case-variants)). A macOS image
collected on Linux or Windows records its launchd property list hints
without a value (`no plutil`), since only macOS has `plutil`.

## Access times

A file's last-access time (`atime`) is the time it was last read. Every
collected file's manifest row records the access time taken before the run
read it, and so does the row of a file collected for a relocation hint,
whose time is taken before the search read it. What a run does to the time
on disk depends on the platform.

### Linux and macOS

Files are always opened for reading alone. Whether a read updates the
access time is the volume's choice, set by its mount options (on Linux,
`relatime`, `noatime` and `strictatime`); listing a directory may update
the directory's access time the same way. The staged copy is given the
source's access and modification times. A file the run reads without
collecting it keeps no record of its old access time: a file read to find
projects that no catalog line names, and a startup file, or with
`--env-scan-home` a swept file, that holds no relocation variable. On a disk
image, mount it read-only, with `noatime` where the file system offers it,
so no read changes anything. `--no-preserve-atime` is accepted and changes
nothing.

### Windows

Whether Windows updates a file's access time when the file is read is a
system-wide setting that varies by machine: the registry value
`NtfsDisableLastAccessUpdate` under
`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`, which
`fsutil behavior query disablelastaccess` shows (see Microsoft's
[`fsutil behavior` reference](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/fsutil-behavior)).
The run follows the stored value. A change to it takes effect only after a
restart, so on a machine changed since its last restart the stored value
may not be what Windows does.

The run reads that value once at the start and decides how to open every
file it reads: each file it copies, and each file it reads to find projects
or relocation hints (with `--env-scan-home`, every file it searches). The
third line of the log says which case applied:

| Log line | What the run does |
| --- | --- |
| `last-access updates are off for NTFS on this system; files on NTFS are opened read-only, others with the right to hold their access time` | Reading a file on a local NTFS volume does not change its access time, so such a file is opened for reading alone. The setting governs local NTFS volumes only: a file on another file system (ReFS, as on a Windows 11 Dev Drive, FAT or exFAT), on a remote volume (a mapped drive or a share, whose server keeps its own times), or on a volume whose file system cannot be read is opened as in the next case. The run first opens the file for reading and asks that open which volume the file is on, so a file reached through a junction or symlink is judged by the volume the link leads to. |
| `last-access updates are on; files are opened with the right to hold their access time` | Each file is opened with the right to change its attributes as well as to read it, and the run tells Windows not to update the file's last-access time for its own read, so the access time on disk stays what it was before the collection. The run changes no file's contents and never sets a file's access time. Where Windows refuses that open (a file the account may only read), the file is read normally, and its access time may change. |
| `last-access updates could not be determined; files are opened with the right to hold their access time` | The value is missing or cannot be read or understood; as the previous case. |
| `--no-preserve-atime: files are opened read-only; reading may change their access time on disk` | The option was given. |

The hold covers files the run reads, not directories: listing a directory
may update that directory's access time where updates are on.

The request to hold the access time is a special value passed where a time
would go. Windows's own file systems and Windows file servers take it as
the request; a server or driver that stored it as a time instead would
overwrite the file's access time with a date far in the future or, where it
reads the value as the signed number -1, a date just before the year 1601.
So after asking, the run reads the access time back on the same open file
and compares it with the value it read just before asking. A change is
logged and left as it is: the run never sets a file's access time, since it
cannot tell a stored request from another program's read between the two
readings, and setting the time would change the file's metadata again. The
check relies on the time the file system reports: on a share, that may come
from the client's cache rather than the server, and a stored request there
would go unnoticed.

Every file read without the hold the run wanted gets a log line, written
when the file is opened, once for each file and reason:

```text
access time not held: <path>: <reason>
```

| Reason | What happened |
| --- | --- |
| `right to hold refused` | Windows refused the open with the right to change attributes, so the file was read with a read-only open. |
| `hold refused` | The file system refused the request to hold the access time; the file was read on the same open. |
| `hold not verified` | The access time could not be read before or after the request; the request stands, unchecked. |
| `access time changed while the hold was asked for` | The access time changed between the two readings: another program's read, or a file system that stored the request as a time (a date far in the future, or just before 1601). It was left as it was, and the file was read on the same open. |

After the last file is read the log adds `access times not held: <N> file(s)`,
the number of distinct files listed, when there is at least one; it is
written also when the run stops early on an error or an interrupt. With
`--inventory`, which writes no log file, both lines go to standard error
with the run's other progress lines unless `-q` is given. Neither line is
written under `--no-preserve-atime`, nor for a file on a local NTFS volume
where updates are off, since no hold was wanted. A refused right or hold is
not listed for a file on a volume that reports itself read-only (a disk
image mounted read-only, a shadow copy), where no read can move an access
time. A refused right or hold on a share is listed even when the share
reports itself read-only, since the server's own disk may still record the
read.

For a listed file, treat the access time now on disk as possibly the run's
own: its read, or the value the file system stored for the request. The manifest's `atime` of a collected file was taken
before the run read it and is the time to use; a listed file read only to
find projects or relocation hints has no manifest row, so its earlier
access time is not recorded.

A local NTFS volume is told by the name of its file system. A file system
that reports the name NTFS without being Windows's own NTFS driver, as some
user-mode file systems do, is treated as one: where last-access updates are
off, its files are opened for reading alone, without the hold, and are not
listed in the log.

Asking for the right to change attributes may trigger security software:
an EDR platform can watch for a program that opens many files with a right
to modify them. If it does, create a behaviour-specific exclusion in that
platform for the file hash of the `doubleagent` executable in use. A hash
(SHA-256, a fingerprint of the file's bytes) names one exact build, so the
exclusion stops matching when the build changes and has to be renewed for
each new version; the release's `SHA256SUMS` lists it, and `Get-FileHash`
computes it on the host.

`--no-preserve-atime` opens every file for reading alone, without the right
to change attributes, whatever the setting says. Use it only when the
security software cannot be tested before a live collection and you accept
the trade-off: reading a file may then change its access time on disk. The
original access time is still recorded for each file the run collects, as
above. A file the run reads without collecting it has no manifest row, so
its old access time is not recorded anywhere: a file read to find projects
that no catalog line names, and a startup file or swept file that holds no
relocation variable. With `--no-preserve-atime` and `--env-scan-home`
together, every swept file that holds no relocation variable may have its
access time changed with no record of the old one. The choice is recorded
in `collection.json` as `options.no_preserve_atime` and at the end of the
log's second line as `no_preserve_atime=0` or `no_preserve_atime=1`.
