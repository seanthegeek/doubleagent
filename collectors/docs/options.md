# Options

Both collectors take the same options, spelled as `--long` flags in the sh
script and as `-Parameter` names in the PowerShell one, and a run with no
options collects every user on the live host into the current directory.
The options below are every one, with what each changes in the output,
the exit codes a console task can test, and the environment variables
each script reads; the short table in the
[collectors README](../README.md#options) is enough for a first run.

## Option reference

| Option | Meaning |
| --- | --- |
| `-o, --output DIR` | Where to write the archive, manifest, summary and log. Created if missing. Default: current directory. |
| `-r, --root DIR` | Alternate root such as a mounted disk image. Switches to image mode. `-r /` is live mode in the sh collector; the PowerShell collector runs in image mode for any `-Root`, `/` and `C:\` included. |
| `-u, --users LIST` | Comma-separated usernames to collect. Default: every home directory found. Names are compared with the user name from the password database, or with the directory name for a home found by globbing. The sh collector compares case-sensitively and the PowerShell collector does not. Also limits the rootless Docker directories. |
| `-p, --project DIR` | Extra project directory whose project-level artifacts (`PROJECT_CATALOG`) are collected, as for a discovered project. Repeatable. It is a path on the machine running the collector and is never prefixed with the root, so in image mode give it including the mount point. A relative path is resolved against the current directory, in image mode too (sh: `cd` and `pwd` for an existing directory, otherwise joined to `pwd` as given; PowerShell: against the current location). A value given twice is collected once. A value that is not a directory (missing, a file, or not visible) is logged as `WARNING: project given with -p is not a directory: <path>: <error>` (`-Project` in the PowerShell message) and gets one `error_read` row; it is not listed in `collection.json` `projects`, only in `options.projects`. Has no effect with `--no-projects`. |
| `--full` | Disable the default size exclusions (model blobs, caches, extension and daemon binaries, marketplace clones). |
| `--no-secrets` | Skip credential files. By default they are collected and flagged `secret: true` in the manifest. |
| `--no-projects` | Skip project-level artifact discovery and `-p`. |
| `--no-docker` | Skip Docker and Podman named volume enumeration (see [docker.md](docker.md)). |
| `--max-file-size MB` | Skip individual files larger than this whole number of MiB. Default 256, `0` disables. |
| `--env-scan-home` | After searching the startup files for relocation variables, also search every regular file in each home and project that the catalog did not match, an exclusion did not prune and the startup file search did not already read, for the same names; a project inside a home is searched once, as a project, a project that is or holds a home directory (`/`, `/home`) is not swept, and the run's own log, manifest and staging directory are never read. Reads only; symlinks are never followed. See [relocation.md](relocation.md#search-the-whole-home). PowerShell: `-EnvScanHome`. |
| `--env-scan-max-size MB` | With `--env-scan-home`, skip files larger than this whole number of MiB and count them as `files_skipped_size`. Default 1, `0` disables. Accepted and recorded without `--env-scan-home`. PowerShell: `-EnvScanMaxSizeMB`. |
| `-k, --keep-staging` | Keep the staging directory, `.stage-<host>_<UTC time>_agent-artifacts` in the output directory, which holds the unpacked archive contents. |
| `--inventory` | Write nothing: walk the catalog with `lstat` only and print JSON Lines to stdout, one host line and one line per user and agent found. `-o` is ignored, and `-k`, `--no-secrets`, `--max-file-size`, `--env-scan-home` and `--env-scan-max-size` have no effect, and no relocation hint is searched for. See [inventory.md](inventory.md). PowerShell: `-Inventory`. |
| `--list` | Print the tables (home catalog, project catalog, exclusions, credential patterns, Docker volume names, relocation variables, startup files), then exit. |
| `-q, --quiet` | Print only the final summary. Progress lines still go to the log file. |
| `-V, --version` | Print `collect-agent-artifacts <version>` and exit. PowerShell: `-Version`. |
| `-h, --help` | Print the usage text and exit. sh only. |

## PowerShell parameters

The PowerShell parameters mirror the sh options: `-OutputDir`, `-Root`,
`-Users`, `-Project`, `-Full`, `-NoSecrets`, `-NoProjects`, `-NoDocker`,
`-MaxFileSizeMB`, `-EnvScanHome`, `-EnvScanMaxSizeMB`, `-KeepStaging`,
`-Inventory`, `-List`, `-Quiet`, `-Version`. `-o`, `-r`, `-u`, `-p`, `-k`
and `-q` are accepted as aliases.
`-Users` takes one comma-separated string. `-Project` takes a PowerShell
array (`-Project C:\src\a,C:\src\b`) rather than a repeated parameter. That
array form needs the script to be started from PowerShell, for example
`powershell.exe -ExecutionPolicy Bypass -Command "& .\Collect-AgentArtifacts.ps1 -Project C:\src\a,C:\src\b"`:
under `powershell.exe -File` the arguments are not parsed as PowerShell, so
`C:\src\a,C:\src\b` arrives as one path, which is not a directory and gets
an `error_read` row. There is no `-h`.
`Get-Help .\Collect-AgentArtifacts.ps1 -Detailed` prints the parameter
help.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | An archive was written, even if individual files failed to copy (manifest status `error_copy`) or directories could not be read (`error_read`). With `--inventory`, the walk ran, even if homes or Docker data roots were unreadable. Also `--list`, `--version` and `--help`. |
| `1` | Usage error: unknown option, missing option value, a `--max-file-size` or `--env-scan-max-size` that is not a whole number (`Invalid --env-scan-max-size: <value>`). PowerShell: a negative `-MaxFileSizeMB`, an `-EnvScanMaxSizeMB` that is not a whole number (`Invalid -EnvScanMaxSizeMB`; the parameter is a string, so a value such as `x` reaches this check), or a parameter binding error. |
| `2` | Fatal: the root is not a directory (`Root is not a directory: <dir>`), the output directory cannot be created (`Cannot create output dir: <dir>`, also when it is a file or lies under one) or written (`Output dir not writable: <dir>`), `tar` or `find` is missing (sh), the staging directory cannot be created (sh), the `CAC_ARCHIVER` test hook names an unknown archiver or leaves none to try (PowerShell, see [testing.md](testing.md)), or no archive could be written. With `--inventory` only the root and `find` checks apply. |
| `130` | sh only: interrupted by `INT` or `TERM`. The staging directory is removed unless `-k` is given. |

## Environment

The sh collector reads `COLLECTOR_SH`, the shell used to run the per-file
workers that `find -exec` starts (recorded as `capabilities.worker_shell`).
When it is unset or empty the workers run under `dash` if `dash` is on the
`PATH` (Debian and Ubuntu, and `/bin/dash` on macOS 10.15 and later), and
under `sh` otherwise; dash runs the workers nearly twice as fast as bash,
which is macOS's `sh`. `CAC_NO_BATCH=1` is a test hook that makes the
workers stat, copy and hash one file at a time instead of in batches; the
manifest is the same either way. The sh collector sets `LC_ALL=C`, puts
`/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin` ahead of the inherited
`PATH`, and unsets `CDPATH`, so a relative `-r` or `-o` is always resolved
against the current directory.

The PowerShell collector reads `COMPUTERNAME` for the host name (falling
back to the DNS host name), `SystemDrive` for the drive whose `Users` and
`home` directories are globbed and whose Docker paths are tried in live mode
(default `C:`), `ProgramData` for the live Docker volume and Docker Desktop
paths, `PROCESSOR_ARCHITECTURE` for `collection.json` `platform`,
`SystemRoot` to find `cmd.exe` for `mklink` when .NET has no
`CreateSymbolicLink` (Windows PowerShell 5.1), and `OS` (it uses `tar.exe`
only when that is `Windows_NT`). `CAC_ARCHIVER` is a test hook that sets
the archivers to try; see [testing.md](testing.md).
