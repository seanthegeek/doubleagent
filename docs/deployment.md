# Deployment through an EDR

During an incident the collector usually reaches a host through the
remote shell of the EDR (endpoint detection and response) product the
responder already has, not a terminal on the host: the responder uploads
the one script file, runs it once, and retrieves the archive it wrote, or
runs it in inventory mode across the fleet and keeps what it printed. The
consoles hold the script's output until it ends and bound how long one
command may run, so a collection through one is planned around those
limits.

## Collection

An EDR product's remote shell is the usual way to reach a host:
CrowdStrike Falcon RTR (Real Time Response),
SentinelOne RemoteOps (Remote Operations, which runs a script on a host as
a console task), Microsoft Defender for Endpoint Live Response, or Palo
Alto Networks Cortex XDR (extended detection and response) Live Terminal.

The script is a single file with no interactive prompts, reads nothing from
stdin, and writes everything under `-o` (`-OutputDir` in PowerShell).
Upload the script, run it with `-o` pointing at a directory you can
retrieve from, then pull the archive (`.tar.gz`; `.tar` or `.zip` after an
archiver fallback, see [collection-output.md](collection-output.md#archiver-fallbacks)). Use `-q`
(`-Quiet`) to keep the console output to the final summary.

On Windows, files held open by a running editor still copy, and paths
longer than 260 characters are collected without any host setting, under
Windows PowerShell 5.1 as well; see
[windows.md](windows.md#locked-files-and-long-paths).

### Console output arrives at the end

The collector writes its progress as it goes: one line per user, catalog
match and project, to stderr unless `-q` is given and always to the log
file under `-o`, one line at a time. In a local terminal that progress is
live.

The EDR consoles hold it back: they run a script as one command whose
result comes back when it ends. Through RTR `runscript`, no output
appeared until the script had finished (observed in October 2026,
[issue 100](https://github.com/seanthegeek/doubleagent/issues/100)). A
long collection therefore shows nothing until the summary; that is the
console's behaviour, not a hang. `-q` changes nothing here: it removes
the progress lines from stderr, but the console holds stdout and stderr
alike until the end.

The consoles also bound how long one command may run:

| Console | Command limit | Source |
| --- | --- | --- |
| CrowdStrike RTR `runscript` | Set with `-Timeout=<seconds>`. The run-script command of the Falcon integration for Cortex XSOAR (Palo Alto Networks' security orchestration, automation and response platform) documents a maximum of 600 seconds (default 30). The RTR session itself times out after 10 minutes unless you interact with it. | [Cortex XSOAR's CrowdStrike Falcon integration reference](https://xsoar.pan.dev/docs/reference/integrations/crowdstrike-falcon) (the 600-second maximum); a CrowdStrike maintainer in [CrowdStrike/psfalcon discussion 249](https://github.com/CrowdStrike/psfalcon/discussions/249), who sets a 600-second command timeout and recommends launching a long script as a separate process and collecting its results afterwards; the [UAC documentation on Falcon RTR](https://tclahr.github.io/uac-docs/using_uac_with_cs_falcon_rtr/) (the session timeout, and a background run of UAC, the Unix-like Artifacts Collector, watched through its log) |
| Defender Live Response `RunScript` | Times out after 10 minutes. | [Run live response commands on a device](https://learn.microsoft.com/en-us/defender-endpoint/api/run-live-response) |
| SentinelOne RemoteOps script task | Set per task in the console; the script's output is read from the task's status in the console once the task has run. | A [Safety deployment guide for SentinelOne](https://docs.safetycli.com/safety-docs/deployment/deployment/sentinelone), which recommends a timeout of at least 1800 seconds for its own long-running script |

(Sources fetched 2026-10-15.)

### Long or watched collections

When a collection may run longer than the console's command limit, or you
want to watch it, start the collector detached from the console's command
so the command returns at once, then read the log from the same session.
This is a manual pattern; a built-in option for it is tracked in
[issue 101](https://github.com/seanthegeek/doubleagent/issues/101).

On macOS, Linux and the BSDs, through RTR `runscript -Raw` or the
console's equivalent (the paths are examples):

```sh
nohup sh /tmp/ir/collect-agent-artifacts.sh -o /tmp/ir -q >/dev/null 2>&1 &
```

On Windows, give the script's full path; a path with spaces needs its own
quotes inside the argument string:

```powershell
Start-Process powershell.exe -WindowStyle Hidden -ArgumentList '-ExecutionPolicy Bypass -File C:\ir\Collect-AgentArtifacts.ps1 -OutputDir C:\ir -Quiet'
```

The log is created in the output directory when the run starts, under
the name [collection-output.md](collection-output.md#output-files) gives,
`<host>_<UTC time>_agent-artifacts.log` (for example
`host_20261003T165531Z_agent-artifacts.log`). The lines written during
the walk start with a UTC timestamp. Read it with `cat` or `tail` (RTR's
`cat` on Windows, or `Get-Content -Tail` in PowerShell) as often as you
like:

- One line per user with agent state, per catalog match (the agent
  name in brackets, then the matched path) and per project is written
  as the walk reaches it, so the lines show which home and agent the
  collector is on. The log does not keep a running count of files; the
  counts are in the summary at the end.
- `Archiving` marks the end of the walk; the archive is being written and
  is not complete yet.
- The `done:` line, `<UTC time> done: <archive path> (<size> bytes, sha256
  <hex>)`, is written once the archive is complete and hashed, as
  [collection-output.md](collection-output.md#stdout-summary) describes. Pull the archive only
  after it appears. It is the last line, except that the PowerShell
  collector adds one more line when it cannot remove its staging
  directory.

Then `get` the archive and its `.sha256` file. With the output sent to
`/dev/null` the stdout summary is not kept, but the same counts are in
`collection.json` beside the archive; see
[collection-output.md](collection-output.md#collectionjson). A run that stops before it creates
the log (a usage error, an output directory that cannot be created)
leaves no log at all, and its message went to the discarded stderr; run
the same command in the foreground to see it.

### Runtime

A run on a developer workstation with several agents installed takes well
under a minute. On a heavy user's host it takes longer: the time grows
with the number and size of the files collected, since each one is copied
and hashed, and long agent histories (years of transcripts, large
per-project histories, Docker volumes) can make it minutes rather than
seconds. Plan for the console limits above, and use the detached pattern
when in doubt.

## Fleet inventory

For a fleet inventory, run the script with `--inventory` (`-Inventory`) and
no `-o`: nothing is uploaded back or left on the host, and the console
output of the run is the result. Save each host's stdout as its own file
(the EDR console's output export or a copy and paste of the response) and
pass the files to the analyzer's `inventory` command. Add `-q` so stderr
stays empty; stderr never mixes into stdout, but some consoles show both
together. The lines are described in [collect-inventory.md](collect-inventory.md).

## Access times

Copying a file updates its `atime` on filesystems that track it. The
manifest records the pre-copy `atime` from `lstat`, taken before the copy.
On a disk image, mount it read-only with `noatime`.

### Access times with the `doubleagent` binary

A file's last-access time (`atime`) is the time it was last read. Whether
Windows updates it when a file is read is a system-wide setting that
varies by machine: the registry value `NtfsDisableLastAccessUpdate` under
`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`, which
`fsutil behavior query disablelastaccess` shows (see Microsoft's
[`fsutil behavior` reference](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/fsutil-behavior)).
The binary follows the stored value. A change to it takes effect only
after a restart, so on a machine changed since its last restart the stored
value may not be what Windows does.

On Windows the `doubleagent` binary reads that value once at the start of
a run and decides how to open every file it reads: each file it copies,
and each file it reads to find projects or relocation hints (with
`--env-scan-home`, every file it searches):

- **Updates are off.** Reading a file on a local NTFS volume does not
  change its access time, so the binary opens it read-only and asks for no
  other right. The setting governs local NTFS volumes alone: a file on
  another file system (ReFS, as on a Windows 11 Dev Drive, FAT or exFAT),
  on a remote volume (a mapped drive or a share, whose server keeps its own
  times), or on a volume whose file system cannot be read, is opened as in
  the next case. The binary first opens the file read-only and asks that
  open which volume the file is on, so a file reached through a junction or
  symlink is judged by the volume the link leads to; a file that is not on
  a local NTFS volume, or whose first read-only open fails, is then opened
  again as in the next case.
- **Updates are on, or the value cannot be read or understood.** The
  binary opens each file with the right to change the file's attributes as
  well as the right to read it, and tells Windows not to update the file's
  last-access time for the collector's own read. This helps protect
  forensic integrity: the access time on disk stays what it was before the
  collection. The binary changes no file's contents, and sets a file's
  access time only to put back a value it read, as described below. Where
  Windows refuses that open (a file the account may only read), the binary
  reads the file normally, and that file's access time may change.

The log says which case applied, in the line after the two start lines.
The hold covers files the binary reads, not directories: listing a
directory may update that directory's access time where updates are on.

The request to hold the access time is a special value passed where a
time would go. Windows's own file systems and Windows file servers take it
as the request; a server or driver that stored it as a time instead would
overwrite the file's access time, with a date far in the future, or, where
it reads the value as the signed number -1, a date just before the year
1601. So after asking, the binary reads the access time back on the same
open file and compares it with the value it read just before asking. It
sets the time back only when the value read back differs from the one
read before and cannot be the time of any real access: the request value
itself, a date before 1980 (the earliest date FAT can hold, and one a
correctly set clock never gives), or a date more than 50 years ahead of
this machine's clock (more than any clock error), counted from
15 November 2026 when this machine's clock reads earlier, as an unset clock
does. It then sets the access
time back to the value read before, passing no other time, reads it again
to check, and reads the file on the same open. Setting the access time is
a change to the file's metadata: it may also update the file's change time
(the time its record last changed; setting the value a file already had
did on one Windows machine and not on another) and may add a record to the
volume's change journal. Any other change between the two readings is
taken as another program's read through its own open, stamped by a clock
that may be far from this machine's, and is left as it is. A server that
stores the request as a date between 1980 and 50 years ahead (some clamp
dates to 2038) is not caught. A device with no clock stamps 1970 until its
clock is set: if such a device's access lands between the two readings,
the binary takes it for a misread and sets the time back over it, the one
case in which a restore can overwrite a real access. The check relies on the time the file system
reports: on a share, that may come from the client's cache rather than the
server, and a misread there would go unnoticed.

Every file read without the hold the run wanted gets a log line, written
when the file is opened, once for each file and reason:

```text
access time not held: <path>: <reason>
```

| Reason | What happened |
| --- | --- |
| `right to hold refused` | Windows refused the open with the right to change attributes, so the file was read read-only. |
| `hold refused` | The file system refused the request to hold the access time; the file was read on the same open. |
| `hold not verified` | The access time could not be read before or after the request; the request stands, unchecked. |
| `access time changed while the hold was asked for` | The access time changed between the two readings to a value a real access could have written (from 1980 to 50 years ahead of this machine's clock, or of 15 November 2026 if that is later): most likely another program's read. The binary left it as it was and read the file on the same open. |
| `hold not honoured, access time restored` | The access time changed, when the hold was asked for, to a value no real access could have written (the request value, a date before 1980, or a date more than 50 years ahead of the later of this machine's clock and 15 November 2026); the binary set the access time back to the value read before, checked it, and read the file on the same open. |
| `hold not honoured, restore failed` | The access time changed, when the hold was asked for, to a value no real access could have written, and could not be set back (it did not read back as set, or the value read before could not be sent as a time); or the first reading failed and the value read back was the request value itself, so the old time is unknown. The file was read on the same open. |

After the last file is read the log adds
`access times not held: <N> file(s)`, the number of distinct files listed,
when there is at least one; it is written also when the run stops early on
an error or an interrupt. With `--inventory`, which writes no log file,
both lines go to standard error with the run's other log lines unless `-q`
is given. Neither line is written under `--no-preserve-atime`, nor for a
file on a local NTFS volume where updates are off, since no hold was
wanted. A refused right or hold is not listed for a file on a volume that
reports itself read-only (a disk image mounted read-only, a shadow copy),
where no read can move an access time. A refused right or hold on a share
is listed even when the share reports itself read-only, since the server's
own disk may still record the read.

For a listed file, treat the access time now on disk as possibly the
binary's own: its read, or with `restore failed` the value the file system
stored for the request. The manifest's `atime` of a collected file was
taken before the binary read it and is the time to use; a listed file read
only to find projects or relocation hints has no manifest row, so its
earlier access time is not recorded.

The binary tells a local NTFS volume by the name of its file system. A
file system that reports the name NTFS without being Windows's own NTFS
driver, as some user-mode file systems do, is treated as one: where
last-access updates are off, its files are opened read-only, without the
hold, and are not listed in the log.

Asking for the right to change attributes may trigger security software:
an EDR platform can watch for a program that opens many files with a right
to modify them. If it does, create a behaviour-specific
exclusion in that platform for the file hash of the DoubleAgent executable
in use. A hash (SHA-256, a fingerprint of the file's bytes) names one
exact build, so the exclusion stops matching when the build changes and
has to be renewed for each new version. Compute the hash on the executable
itself, with `Get-FileHash` in PowerShell (it uses SHA-256 by default) or
with `sha256sum` or `shasum -a 256` on Linux and macOS.

The `--no-preserve-atime` option makes the binary open files read-only,
without the right to change attributes, whatever the setting says. Use it
only when the security software cannot be tested before a live collection
and the operator accepts the trade-off: reading a file may then change its
access time on disk. The original access time is still recorded for each
file the binary collects: the manifest's `atime` is taken before the file
is copied, and for a file the relocation search reads, before that read.
A file the binary reads without collecting it has no manifest row, so its
old access time is not recorded anywhere: a file read to find projects
that no catalog line names, and a startup file or swept file that holds no
relocation variable. With `--no-preserve-atime` and `--env-scan-home`
together, every swept file that holds no relocation variable may have its
access time changed with no record of the old one. The choice is recorded in
`collection.json` as `options.no_preserve_atime` and at the end of the
log's second line as `no_preserve_atime=0` or `no_preserve_atime=1`.

On Linux, macOS and other Unix systems the option is accepted and has no
effect in this version: the binary always opens files read-only there and
logs no such decision. Reading a file may update its access time as the
volume's mount options decide, and the manifest records the time taken
before the binary read the file, as above.
