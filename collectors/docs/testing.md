# Testing

The collectors are tested end to end. `tests/smoke.sh` and `tests/smoke.ps1`
build a fake disk image with two users, awkward file names, credentials,
excluded directories, an oversized file and symlinks, run the collector
over it in image mode and check the manifest rows, the archive and each
option; `tests/catalog-sync.sh` checks that the five catalog tables agree
between the two scripts and the analyzer's copy. A change to a collector
runs the sh smoke test under every shell the script must support, the
PowerShell one under PowerShell 7 and Windows PowerShell 5.1, and the drift
check before it is pushed, and CI runs the same matrix.

Run from the `collectors/` directory:

```sh
tests/smoke.sh          # under sh
tests/smoke.sh dash
tests/smoke.sh bash
printf '#!/bin/sh\nexec busybox ash "$@"\n' > /tmp/ash && chmod +x /tmp/ash && tests/smoke.sh /tmp/ash
printf '#!/bin/sh\nexec zsh --emulate sh "$@"\n' > /tmp/zsh-sh && chmod +x /tmp/zsh-sh && tests/smoke.sh /tmp/zsh-sh
tests/smoke.sh posh
shellcheck -s sh -S warning collect-agent-artifacts.sh
tests/catalog-sync.sh   # the five tables, --list output and the analyzer copy agree
pwsh -NoProfile -File tests/smoke.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests\smoke.ps1   # on Windows
```

The linters CI runs on every file (shfmt, shellcheck with `.shellcheckrc`,
checkbashisms, PSScriptAnalyzer, markdownlint) are listed under "Quality
gates" in [AGENTS.md](../../AGENTS.md#quality-gates).

## Smoke tests

`tests/smoke.sh [SHELL]` runs the collector script under `SHELL` (default
`sh`, a name on the `PATH` or a path), and passes the same shell to the `find
-exec` workers through `COLLECTOR_SH`. Every collection run is in image mode
with `--max-file-size 1` except three: a live run limited to a user that does
not exist (`-u no-such-user-cac --no-docker --no-projects`), which checks that
no `live/` directory or `live` row appears; a run with a relative `-r` and
`CDPATH` set, which checks that the image in the working directory is
collected; and, when not run as root, a run into a read-only output directory,
which must exit `2` before collecting. A run with `-o` under a file must exit
`2` the same way. It works in a `cac-smoke.*` directory under `TMPDIR`
(default `/tmp`). The directory is removed when every check passes and kept,
with its path printed, when one fails. The exit status is `0` or `1`.

`tests/smoke.ps1` takes `-Collector PATH` to test a copy of the script other
than the one beside it, which is how the test runs under Windows PowerShell
5.1 from a WSL checkout. It works under the system temp directory, keeps it
on failure in the same way, and skips the symlink checks, saying so, where
creating symlinks is not permitted. Its symlink-in-archive checks use a
separate small image with a relative, a dangling and a directory symlink
(and, on Windows, a junction) and run it four times: with `tar`, with the
PowerShell tar writer, with the zip from the start, and, off Windows, with a
`tar` that fails and the zip after it (the late fall back to zip).

`CAC_ARCHIVER` is a test hook for the PowerShell collector that sets the
archivers to try, in order, as a comma-separated list of `tar`, `writer`
(the PowerShell tar writer) and `zip`; the default is `tar,writer,zip`. An
unknown name exits `2`, `tar` is dropped when no `tar` is on the `PATH`,
and the archive is a `.zip` only when `zip` comes first, in which case no
symlinks are staged. The smoke test uses `writer` and `zip` to run each
archiver on every platform, and `tar,zip` with a failing `tar` for the late
zip fallback. It also collects an image with Latin-1, Cyrillic and CJK file
names and a symlink with a Cyrillic name: on Windows `tar.exe` fails on it,
and the test checks the PowerShell tar writer's archive, `archiver`, the
note and the log line, and reads the archive with its own .NET tar reader
(header checksums, pax records, member names, link targets and hashes),
since `tar.exe` cannot list those names. With the writer forced, it checks
that the platform `tar` lists the same members as the default run and
extracts bytes that match the manifest, and with the zip forced that the
entry names use `/`.

Each smoke test builds a fake disk image with two users, a service account,
awkward filenames, a symlink, credential files, excluded directories, an
oversized file and projects referenced from agent state, then checks the
manifest, hashes and archive contents. The sh test also has a `nobody`
account to skip. It is POSIX sh too, and its JSON validity and hash
cross-check steps run only when `python3` is available. The PowerShell test
uses a Windows profile tree with drive-letter and `file:///` project
references and runs under both PowerShell 5.1 and 7. Both tests read
project folders from Antigravity project files with a POSIX or
drive-letter `file:///` URL.

Both smoke tests also build root and rootless Docker volumes, one matched
and one unmatched, check `--no-docker` and the Docker Desktop note, and run
`--no-secrets`, `--full` and `-u`. The sh test adds an unreadable volume and
data root when it is not run as root. Both run `--inventory` / `-Inventory`
against the same image and check the key order of both line types, one
known line's `files`, `bytes`, `first`, `last` and `evidence`, that an
excluded subtree is not counted (and is with `--full`), the Docker volume
lines, `-u`, `--no-docker`, that nothing is written in the current directory
or under `-o`, that stdout stays pure JSON Lines with and without `-q`, and,
on a POSIX host not run as root, that an unreadable home is counted in
`users_unreadable`.

The sh test also runs a small separate image through the batched stage
worker, which lstats, copies and hashes up to 256 files at a time: names
with a newline, a backslash and a carriage return, an unreadable file
between readable ones (not when run as root), and 300 files, more than one
chunk. It checks the hashes against the archived bytes, the `error_copy`
row and log line, that a run with `CAC_NO_BATCH=1` (the per-file path)
writes the same manifest in the same order apart from `atime`, and that a
copy of the collector forced onto `shasum` writes the same manifest.

## Table coverage checks

Two checkers compare each line of the five catalog tables with what
should exist for it elsewhere in the repository. Run them from the
repository root; both use the standard library alone:

```sh
python3 tools/check_tables_research.py   # each line against its agent's research document
python3 tools/check_smoke_coverage.py    # each line against both smoke tests
```

`tools/check_tables_research.py` reports a line whose glob no code span of
the Collection part of the agent's research document names, a credential
file that document names that no secret glob flags, a secret glob an
exclusion hides, an exclusion or secret glob that no home or project entry
reaches and no one Docker volume agent's document names, a Docker volume
glob that the document's Per-user storage subsection about the volume does
not name, an exclusion below a Docker volume's mount directory without its
volume-relative twin, two
agents' home entries that differ only by case or can match one path, a
Credentials section saying a file is left unflagged instead of naming its
secret glob, a timeline row text or record key under Collection, a
catalog agent without a research document or a row on the
[agents page](../../docs/agents.md), and a row on that page naming no
catalog agent.

`tools/check_smoke_coverage.py` requires, in both smoke files, a
default-run check asserting `secret: true` and a `--no-secrets` check
asserting `skipped_secret` on a path each `SECRET_GLOBS` line matches, a
check asserting `skipped_excluded` for each `EXCLUDES` line, and for each
`CATALOG` and `PROJECT_CATALOG` line a fixture path it matches whose name
the research document gives. It reads both smoke tests as text: keep
fixture paths and check arguments as literals, variables assigned from
literals, `kind|agent|path` case lists, or here-document lists read by
`while read`, and write `Row` and `StatusOf` arguments in `smoke.ps1` from
the image root (`Users/alice/...`), never as a suffix.

The disagreements that existed when the checkers were written are
recorded in `tools/drift-baseline/`, and only a finding missing from it
fails; [Drift checkers in `analyzer/docs/development.md`](../../analyzer/docs/development.md#drift-checkers)
describes the finding format and the baseline.

## Catalog drift check and CI

`tests/catalog-sync.sh` extracts the five tables from both scripts and fails
if they differ or if `analyzer/doubleagent/catalog.txt` is not identical to
`--list`. When `pwsh` is on the `PATH` it also compares `--list` with
`-List`. The repository's pre-commit hook in `.githooks/`
(enable it once per clone with `git config core.hooksPath .githooks`)
refuses a commit that stages either collector or the analyzer copy while the
copy is stale or the tables differ; the linters run in CI only.

CI (`.github/workflows/ci.yml`) runs the drift test and shellcheck, the sh
smoke test under sh, dash, bash, busybox ash, zsh and posh, the drift test
and the sh smoke test under dash on macOS, where the tools are the BSD
ones, a lint job that also runs the table coverage checks, the
PowerShell smoke test under PowerShell 7 on Linux and Windows PowerShell 5.1
on Windows, and the analyzer tests, then a `CI result` job that fails when
any of them failed, on every push to `main` and every pull request.
