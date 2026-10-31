# Internals

How the two collectors are built, for a maintainer who has to audit or
extend them. The project's standards for the collectors are in
[AGENTS.md](../../AGENTS.md#non-negotiable-standards), the readability
rules in [its Readability section](../../AGENTS.md#readability).

What the collectors write is documented elsewhere and not repeated here:
the files and fields in [output.md](output.md), what is collected and the
user-visible matching rules in [coverage.md](coverage.md), the volume
behaviour in [docker.md](docker.md), the inventory lines in
[inventory.md](inventory.md), and the Windows archivers, symlinks and long
paths in [windows.md](windows.md).

## Layout

Both scripts are one file each, because an EDR (endpoint detection and
response) console uploads one file and runs it once. Each is in twelve
sections, in the same order and under the same banner titles, so the two
files read in parallel:

| # | Section | sh, `collect-agent-artifacts.sh` | PowerShell, `Collect-AgentArtifacts.ps1` |
| --- | --- | --- | --- |
| 1 | Reader's guide | A comment block under the licence header: the section map, the data flow, the naming rules. | The same, after `Set-StrictMode`, because the comment-based help and `param()` must come first in a PowerShell script. |
| 2 | Data tables | `CATALOG`, `PROJECT_CATALOG`, `EXCLUDES`, `SECRET_GLOBS`, `DOCKER_VOLUMES` as single-quoted strings. | The same tables as `@'...'@` here-strings, byte-identical (`tests/catalog-sync.sh` checks). |
| 3 | Shared helpers | Logging, hashing, `stat`, JSON escaping and number checks, kept as text in `COMMON` (see [Worker programs](#worker-programs-sh)); `heredoc_var`, `add_note` and `file_id` run in the main shell only. | The table reader, logging and notes, JSON escaping, glob-to-regex conversion, extended-length paths and file-system probes that never throw. |
| 4 | Manifest writer | `write_manifest_row`, the directory-read check and the `error_read` row, also in `COMMON`. | `Write-ManifestRow`, which also keeps the per-status counts, `Get-DirReadError`, `Write-ReadErrorRow`. |
| 5 | Staging files | The four worker programs `find -exec` runs. | The per-file copy, hash and row (`Add-CollectedFile`), symlink recreation, excluded rows. |
| 6 | Catalog walk | Glob and catalog expansion, case folding, claimed paths, one match, the `find` walk. | The same steps as functions that walk with `Get-ChildItem`. |
| 7 | Users | Enumerating and filtering home directories. | The same, from the registry's `ProfileList` and the usual home roots. |
| 8 | Docker and Podman volumes | Volume directories found on disk, matched against `DOCKER_VOLUMES`. | The same. |
| 9 | Project discovery | Six source groups, then the per-project walk. | The same six groups. |
| 10 | Inventory output | The `--inventory` JSON lines. | The `-Inventory` accumulators and JSON lines. |
| 11 | Summary and archive | `collection.json`, the archiver chain, the archive hash, the stdout summary. | The same, with the PowerShell tar writer and the zip fallback. |
| 12 | Main program | Option parsing, setup, then one call per step. | Setup functions, then one call per step (the options come from `param()` at the top). |

## Run flow

The data flow is the same on both sides. The home enumeration lists the
homes; for each home the catalog walk expands every `CATALOG` line first,
claims every match (so a match nested in another is collected once, under
the more specific agent), then walks each match. `EXCLUDES` prunes a path
with one `skipped_excluded` row, `SECRET_GLOBS` flags a file `secret`,
every file is stat'ed with `lstat` semantics (the metadata of the file
itself, never of a symlink's target) before it is copied, a symlink is
recreated rather than followed, and the staged copy, not the source, is
hashed. Docker and Podman volumes and the discovered projects are further
bases for the same walk. Then `collection.json` is written and the staging
directory becomes the archive.

The steps in the main program, in run order:

| Step | sh | PowerShell |
| --- | --- | --- |
| Options | `parse_options` (exits after `--version`, `--help`, an unknown option, or exit 1 with the usage for an option missing its value); `--list` runs `print_catalog` and exits 0; `set_max_size` (exit 1 on a bad value) | `param()`; `-Version` and `-List` (`Write-CatalogList`) exit 0; a negative `-MaxFileSizeMB` exits 1 |
| Resolve options | `resolve_extra_projects`, `resolve_root` (exit 2 when `-r` is not a directory) | `Initialize-TableRegexes`, `Resolve-ProjectArgs`, `Resolve-RootArg` (exit 2) |
| Output directory and tools | `check_prerequisites`: outside `--inventory`, the output directory is created and must be writable and `tar` must exist; `find` must exist (exit 2) | `Initialize-OutputDir` outside `-Inventory` (exit 2) |
| Capabilities and names | `detect_capabilities`, `compile_secret_pattern`, `define_secret_test`, `set_output_names` | `Initialize-RunNames`, `Initialize-Archivers` (exit 2 on a bad `CAC_ARCHIVER`) |
| Open outputs | exports the worker settings, sets the `EXIT`, `INT` and `TERM` traps, `create_output_files`, sets `ACTIVE_EXCLUDES` (empty under `--full`), `log_start`, then clears the globals the steps fill (`NOTES`, the `DOCKER_*` counters, `DOCKER_INV`, `PROJECT_LIST`, `NESTED_CLAIMS`) so no value comes from the environment | `Open-RunFiles` |
| Users | `USER_LIST=$(get_user_homes)`, a warning when it is empty, then `collect_user_homes` unless `--inventory` | `Get-UserHomes`, `Test-Administrator`, `Write-StartLog`, then `Add-UserHomes`, which under `-Inventory` is also the inventory walk |
| Volumes | `collect_docker_volumes` unless `--no-docker` | `Add-DockerVolumes` unless `-NoDocker` |
| Projects | `collect_projects` (discovery and walk) unless `--no-projects` | `Find-Projects`, then `Add-Projects`, unless `-NoProjects` |
| Inventory | `write_inventory` (which walks the homes now, after discovery), exit 0 | `Write-Inventory`, exit 0 |
| Summary | `list_active_users`, `sum_collected_bytes` | `Initialize-Summary`, `Write-Summary`, `Close-RunFiles` |
| Archive | `write_archive` (writes `collection.json` and stages it with the manifest and log; exit 2 when no archive was written) | `Write-Archive`; exit 2 when no archive was written |
| Finish | `hash_archive`, `print_result`, exit 0; the `EXIT` trap runs `cleanup` | `Write-ArchiveHash`, `Clear-Staging` unless `-KeepStaging`, `Write-Result`, exit 0 |

The order of the inventory walk differs between the sides: the sh collector
walks the homes in `write_inventory`, after project discovery, and the
PowerShell collector in `Add-UserHomes`, before it. An `INT` or `TERM` signal
makes the sh collector log `interrupted`, run `cleanup` and exit 130.

## Worker programs (sh)

Every walk in the sh collector is one `find(1)`, and `find -exec` cannot
call a shell function: functions are not inherited by the process `find`
starts. The per-file work therefore runs as separate programs, each held in
a variable and started as
`find ... -exec "$WORKER_SH" -c "$PROGRAM" sh ARGS... {} +`. The `{} +`
form passes many paths to one process.

**`COMMON`.** The helpers the main shell and the workers both need (from
Shared helpers: `utc_timestamp`, `log_line`, `hash_file`, `stat_file`,
`stat_files`, `hash_files`, `hash_match`, `json_escape`, `to_number` and
their `_var` forms; from Manifest writer: `write_manifest_row`,
`get_dir_read_error`, `get_dir_read_error_var`, `write_read_error_row`)
are text, read from two here-documents
(`EOF_COMMON_HELPERS`, `EOF_COMMON_MANIFEST`) into the variable `COMMON`.
The main shell `eval`s that text, and each worker program is the text of
`COMMON` followed by the worker's own text, so the helpers are defined the
same way in both places.

**`heredoc_var`.** Each here-document is read with `heredoc_var`, which sets
`_ret_heredoc_text`, instead of `VAR=$(cat <<'EOF' ... EOF)`.
pdksh-derived shells (posh, the Debian Policy-compliant ordinary shell
descended from the public-domain Korn shell) end a `$(...)` at the first
`)`, even a `case` pattern's inside a here-document. For the same reason
any `case` pattern inside a `$(...)` in the file is written with its
opening parenthesis, `(pattern)`.

**`WORKER_SH`.** `detect_capabilities` sets it to `COLLECTOR_SH` when that
is set, else to `dash` when it is on the `PATH` (it starts and loops faster
than bash), else to `sh`. The main program exports the settings the workers
read from the environment: `HASH_TOOL`, `STAT_MODE`, `MAX_SIZE`,
`SECRET_GLOBS`, `NO_SECRETS`, `LOG` and `QUIET`. Everything else is passed
as positional arguments, which the first line of each worker names.

The four workers:

| Worker | `find` gives it | Arguments | Writes |
| --- | --- | --- | --- |
| `STAGE_FILES_WORKER` | every path that is not a directory (`! -type d`): regular files, symlinks and special files | `stage_dir root user home agent manifest files...` | the staged copies, symlinks and one manifest row per file |
| `EXCLUDED_PATHS_WORKER` | every pruned `EXCLUDES` match; also run by `collect_path` for an excluded match that is not a directory | `root user home agent manifest paths...` | one `skipped_excluded` row each: a directory's size from `du -sk` with its other metadata 0, a file's from `stat_file` |
| `UNREADABLE_DIRS_WORKER` | every directory | `user home agent manifest dirs...` | one `error_read` row for each directory `get_dir_read_error_var` says cannot be listed or entered |
| `INVENTORY_FILES_WORKER` | every regular file, under `--inventory` | `agent files...` | `F\|agent\|size\|mtime` lines on stdout, nothing on disk |

**The stage worker's chunks.** `STAGE_FILES_WORKER` takes its files in
chunks of at most 256 paths, and stops adding names to a chunk once their
lengths add up to 65,536 bytes, so no argument list it builds is much
longer than the one `find` passed. Each chunk goes through five passes, in
this order, which keeps the per-file order of the forensic rules:

1. `lstat_chunk` stats every file with one `stat_files` call; when that
   output does not have one line per file (a file vanished, `stat` failed,
   or there is no `stat`), it falls back to `stat_file` per file.
2. `plan_chunk` creates the staging directories with one `mkdir -p`,
   recreates symlinks with `readlink` and `ln -s`, tests `SECRET_GLOBS`
   with `is_secret`, and marks each file `symlink`, `skipped_special`, `skipped_secret`,
   `skipped_size` or `copy`. A path that exists and is neither a symlink
   nor a regular file is `skipped_special`, its `type_N` set to `socket`,
   `fifo`, `device` or `other` by `test -S`, `-p`, `-b` and `-c`; it is
   never copied (see
   [Special files in `output.md`](output.md#special-files)).
3. `copy_chunk` gives each run of consecutive `copy` files from one
   directory to `copy_run`, which copies a run of more than one file with
   one `cp -p`; a run whose `cp` fails or prints anything is redone per
   file, so each failing file gets its own error text and `copy failed`
   log line.
4. `hash_chunk` hashes the staged copies with one `hash_files` call; a
   name the hashers would escape (one holding a backslash, newline or
   carriage return) or a file missing from their output is hashed alone
   with `hash_file`, which gives an empty hash when the copy is
   unreadable.
5. `write_chunk_rows` writes the manifest rows in the order `find` passed
   the files.

POSIX sh has no arrays, so a chunk is held in numbered variables set with
`eval`, for N from 1 to `chunk_size`: `path_N` (the source path), `stat_N`
(its `stat_file` line), `status_N`, `type_N` (the manifest `type`),
`secret_N`, `target_N` (a symlink's target), `copy_error_N` and `hash_N`.
`CAC_NO_BATCH=1`, a test hook, makes passes 1, 3 and 4 and
`INVENTORY_FILES_WORKER` take the per-file path; the manifest is the same
either way.

**Why PowerShell has none.** The PowerShell collector walks with
`Get-ChildItem` in its own process, and `Add-CollectedTree` calls
`Add-CollectedFile` for each file directly. Its Staging files section
keeps the banner and says so.

## Glob matching

| Glob | Matched against | sh | PowerShell |
| --- | --- | --- | --- |
| `CATALOG`, `PROJECT_CATALOG` | the base (home, project), one path segment at a time; `*` and `?` do not cross `/` | shell pathname expansion of `BASE/GLOB` in `expand_glob`, with `IFS` set to newline only so a base with spaces stays one word; a pattern that matches nothing expands to itself and fails the existence test. As in any POSIX shell, `*` does not match a name's leading `.`. | `Expand-Glob` splits the glob on `/`; a wildcard segment lists the directory and matches each name with `Convert-GlobToRegex` (`*` is `[^/]*`, so it matches a leading `.`) and `-match`, which ignores case; a literal segment is tested for existence and respelled as on disk by `Get-OnDiskName`. |
| `EXCLUDES` | the path relative to the base; `*` crosses `/` | `find -path "$base/$glob"` in `collect_tree`, which splits the table with globbing off, and a `case` pattern in `test_excluded` for a match that is not a directory, case-sensitive | `$script:ExcludeRegex`, built by `ConvertTo-TableRegex`, tested in `Add-CollectedPath` and `Add-CollectedTree`; case-insensitive |
| `SECRET_GLOBS` | the path relative to the base; `*` crosses `/` | one `case` pattern, every glob joined with `\|`, built once by `compile_secret_pattern` and turned into `is_secret` by `define_secret_test`; `is_secret` is called in `plan_chunk` (and in `test_inventory_secret`), case-sensitive | `$script:SecretRegex`, built by `ConvertTo-TableRegex`, tested in `Add-CollectedFile` and `Test-InventorySecret`; case-insensitive |
| `DOCKER_VOLUMES` | the volume directory's name | a `case` pattern in `get_docker_volume_rule`, with globbing off | `Convert-GlobToRegex` and `-cmatch`, case-sensitive, in `Get-DockerVolumeRule` |

`[...]` classes and `[!...]` negated classes work everywhere: sh `case`,
pathname expansion and `find -path` take them natively, and
`Convert-GlobToRegex` keeps a class and turns `[!` into the regex `[^`.
`ConvertTo-TableRegex` joins one table's per-glob regexes into a single
anchored alternation, `^(?:b1|b2|...)$`, tested with one `IsMatch` per
path; an empty table (`EXCLUDES` under `-Full`) becomes `(?!)`, which
matches nothing.

The base is the home, the project or the volume's `_data` directory, so one
exclusion such as `.claude/worktrees` applies in each. In the sh collector
the `EXCLUDES` test is part of the `find` expression, so it applies to a
matched directory and everything under it; a match that is not a directory
is never given to `find`, so `collect_path` tests it with `test_excluded`
and gives it to `EXCLUDED_PATHS_WORKER` when it matches. In the PowerShell
collector `Add-CollectedPath` tests the match itself and
`Add-CollectedTree` each child.

## Claimed paths and case variants

A catalog line nested inside another line's match
(`.gemini/antigravity-cli` inside `.gemini`) claims its subtree, as
[coverage.md](coverage.md#nested-entries) describes. Both sides expand the
whole table before anything is walked:

- sh: `expand_catalog` prints `agent|glob|path` for every match, in table
  order. `collect_catalog` sets `NESTED_CLAIMS` to the matched paths for
  the duration of the call, and `collect_tree` adds one
  `( -path CLAIM -o ... ) -prune -o` group to the `find` expression for the
  claims inside the directory being walked. The main program clears
  `NESTED_CLAIMS` before the first step, so a walk that does not go
  through `collect_catalog` (a Docker volume's) prunes no claim, whatever
  the environment held.
- PowerShell: `Expand-Catalog` returns the matches and records each in
  `$script:ClaimedPaths`; `Add-CollectedTree` skips a child that is a key
  there. `Get-ClaimTable` makes that table a case-insensitive hashtable on
  Windows and an ordinal `Dictionary` elsewhere.

Catalog lines that differ only by case can meet one directory on a
case-insensitive filesystem ([coverage.md](coverage.md#case-variants)):

- sh: `classify_case_matches` (awk) compares the matches case-folded and
  emits one instruction per match; `fold_case_matches` then compares the
  device and inode (`file_id`) of the colliding paths only, drops a second
  spelling of the same file with a `#|` log line for `collect_catalog`, and
  respells a match inside another match's differently spelled directory
  with that match's prefix, so the nested claim prunes it. The
  instructions `classify_case_matches` prints, one per match in catalog
  order, are `K|agent|glob|path` (keep it), `@|path` then
  `D|agent|glob|path` (it folds to the earlier match at `path`: drop it if
  it is the same file), and `@|path`, `^|prefix`, `R|agent|glob|rest` (it
  folds to a path inside the match at `path`, spelled differently: respell
  `prefix` as that path's if it is the same directory; the shortest such
  parent is chosen).
- PowerShell: `Get-OnDiskName` returns the on-disk spelling of a literal
  segment, so two such lines expand to the same path, and `Expand-Catalog`
  keeps the first and lists the other with an empty agent, which
  `Add-CatalogMatches` logs and skips.

A directory that a wildcard segment could not list is recorded by
`Expand-Glob` in `$script:GlobReadErrors`; `Write-GlobErrorRows` writes an
`error_read` row for each one that is neither the base nor at or under a
matched path. The sh collector has no equivalent: an unlistable directory
on the way to a match makes the glob match nothing.

## Manifest writer

Every manifest row goes through `write_manifest_row` (sh) or
`Write-ManifestRow` (PowerShell), which write one JSON line by hand rather
than through a JSON library: the sh collector has none, and PowerShell's
`ConvertTo-Json` is slower per row and differs between 5.1 and 7. The
fields and statuses are in [output.md](output.md#manifest).

- sh: the arguments are the manifest path and the 19 fields in manifest
  order. String fields go through `json_escape_var`, which escapes
  backslash, double quote, tab, carriage return and newline and forks
  `awk` only when one is present; numeric fields go through
  `to_number_var`, which turns anything that is not a non-negative integer
  into 0. Rows are appended with `>>`, by the main shell and by the
  workers.
- PowerShell: the parameters are the fields in manifest order without
  `uid`, `gid` and `mode`, which are always 0, 0 and `""` on this side,
  plus `owner` and `attributes`. A row with no control character (or
  U+0085, U+2028, U+2029) escapes `"` and `\` in one pass; otherwise each
  field goes through `ConvertTo-JsonEscaped`, which escapes the way
  PowerShell 7's `ConvertTo-Json` does. The same call counts the row in
  `$script:Counts` (for `collection.json` and stdout) and marks its home in
  `$script:RowHomes`; the sh collector counts afterwards from the manifest
  (`count_status`, `list_active_users`, `sum_collected_bytes`).

**Row order.** The manifest is written in run order:

1. Homes, in enumeration order. In PowerShell, the `error_read` rows of
   denied homes (`Get-DeniedHomes`: listed by their parent but not
   reachable) come first. For each home: its own `error_read` row when it
   cannot be walked; in PowerShell, the `Write-GlobErrorRows` rows; then
   each catalog match in table order. A match that cannot be listed or
   entered gets one `error_read` row and is not walked (sh: in
   `collect_path`; PowerShell: in `Add-CollectedTree`).
2. Docker and Podman volumes: the root Docker, root Podman and
   Windows-container data roots, then each user's rootless Docker and
   Podman roots; within a volumes directory, the volumes in name order
   (sh, glob expansion) or `Get-ChildItem` order (PowerShell).
3. Projects: first a row for each `-p` value that is not a directory, then
   each project in discovery order, its own `error_read` row first when it
   cannot be walked, then its `PROJECT_CATALOG` matches.

Within one match the order follows the walk. PowerShell walks depth first,
children in `Get-ChildItem` order, writing each row as it goes. The sh
`find` hands paths to three workers, and each worker runs when `find` has
filled its argument list or the walk ends, so a walk's rows come in worker
batches; inside a batch they are in `find`'s order.

After a late fall back to the zip archiver, `Clear-LinkArchivePaths`
rewrites the `symlink` rows in place (no `archive_path`, the reason in
`error`); see [windows.md](windows.md#symlinks-and-the-zip-fallback).

## Capability detection

The sh collector detects what the host has once, in `detect_capabilities`,
and records the result in the log and in `collection.json`'s
`capabilities`:

- `HASH_TOOL`: the first of `sha256sum`, `shasum` (if `shasum -a 256`
  works), `sha256` and `openssl` on the `PATH`, else `none` (empty hashes).
  `hash_file`, `hash_files` and `hash_match` branch on it.
- `STAT_MODE`: probed with `stat -c '%s %W' /`. Digits and spaces only
  means `gnu`; other output means `gnu0`, a GNU `stat` without the `%W`
  birth time, which then reads as 0; no output means `bsd` when
  `stat -f '%z' /` works, else `none`, where the size comes from `wc -c`
  and the other fields are 0. `stat_file`, `stat_files` and `file_id`
  branch on it.
- `WORKER_SH`: see [Worker programs](#worker-programs-sh).

The main program puts `/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin` ahead
of the inherited `PATH`, sets `LC_ALL=C` and unsets `CDPATH`, which `cd`
would otherwise search for the relative `-r` and `-o` that `resolve_root`
and `check_prerequisites` resolve with `cd` and `pwd`. Two smoke-test
checks build a patched copy of the script with `sed`: one rewrites the line
that starts `PATH="/usr/bin:`, the other the text `command -v sha256sum `
(with its trailing space), so a change to either string breaks them.

The sh archiver chain is fixed, in `write_archive`: `tar -czf`, then
`tar_gzip` (`tar -cf - | gzip`, with `tar`'s exit status passed through
the `TAR_STATUS_FILE` file since POSIX sh has no `pipefail`) when `gzip` is on the
`PATH`, then an uncompressed `tar`. `write_summary` runs before each try so
`collection.json` names the archive and archiver used.

The PowerShell collector hashes with .NET's SHA-256 and has no `stat`
mode. Its capability choices are made where they are used:

- `Initialize-Archivers` builds `$script:Archivers` from the chain
  `tar,writer,zip` (`tar.exe` or the system `tar`, the PowerShell tar
  writer `Write-TarGz`, a zip from `Write-StageZip`), leaves `tar` out when
  it is not on the `PATH`, and under the test hook `CAC_ARCHIVER` takes
  the chain from that comma-separated value instead. It also decides
  whether links are staged (not when the zip comes first) and the archive
  extension. `Write-Archive` tries each in turn, `Switch-Archiver`
  preparing the next one.
- `Get-LinkApi` picks the symlink API once: `dotnet`
  (`CreateSymbolicLink`, PowerShell 7), `mklink` (through `cmd.exe`,
  Windows PowerShell 5.1), or none.
- `$script:UseExtendedPaths` is set on Windows, where
  `ConvertTo-ExtendedPath` turns every path given to the file system into
  the extended-length form (`\\?\C:\...`), which works past 260
  characters without the `LongPathsEnabled` setting; see
  [windows.md](windows.md#locked-files-and-long-paths).

## Docker and Podman volumes

Volumes are found on disk, never through the `docker` binary, so live hosts
and disk images are handled alike ([docker.md](docker.md) has the
user-facing behaviour). The functions, sh first:

- `collect_docker_volumes` / `Add-DockerVolumes`: the main step. Calls
  `collect_volume_dir` / `Add-VolumeDir` for `var/lib/docker/volumes`,
  `var/lib/containers/storage/volumes` and `ProgramData/Docker/volumes`
  under the root, then for each user's `.local/share/docker/volumes` and
  `.local/share/containers/storage/volumes`, then notes each Docker Desktop
  data directory that exists (PowerShell: `Find-DockerDesktop`).
- `test_volume_root` / `Test-VolumeRoot`: walks the components of the
  volumes directory, so a symlinked or unreadable component gets a note
  instead of silence; an unreadable one is also counted unreadable.
- `collect_volume_dir` / `Add-VolumeDir`: logs `Docker volumes in DIR` and
  gives each real directory in it to `collect_volume` / `Add-Volume`,
  counting it as found.
- `collect_volume` / `Add-Volume`: matches the name with
  `get_docker_volume_rule` / `Get-DockerVolumeRule`. An unreadable volume
  is noted; one without a `_data` directory is logged and skipped; an
  unmatched one gets its `skipped_unmatched_volume` row (PowerShell:
  `Write-UnmatchedVolumeRow`), none under inventory; a matched one has its
  `_data` given to `collect_path` / `Add-CollectedPath` as a single match,
  with user `docker`, the rule's agent, and `_data` as the home, so
  `EXCLUDES` and `SECRET_GLOBS` match relative to it. A matched volume
  whose walk met unreadable paths is noted too: the sh collector counts
  the `find:` lines its walk added to the log (`find_errors_since`), the
  PowerShell collector reads `$script:WalkErrors`.

The sh counters are the globals `DOCKER_SEEN`, `DOCKER_FOUND`,
`DOCKER_COLLECTED`, `DOCKER_UNREADABLE` and `DOCKER_DESKTOP`, with
`DOCKER_INV` holding the inventory lines of collected volumes; every
function that sets them runs in the main shell, never in a pipeline or a
`$(...)`, where the assignments would be lost. The PowerShell collector
keeps the same counts in `$script:Docker` and the inventory in
`$script:InventoryDocker`. A failure (an unreadable volumes directory, a
volume that cannot be walked) is counted, noted in `collection.json` and
printed in the summary, and never stops the collection.

## Project discovery

`discover_projects` / `Find-Projects` read project paths from each home's
agent state with `grep` and `sed` (sh) or a regex per format (PowerShell);
nothing is parsed, and SQLite-only sources are left to the analyzer. Six
source groups run in this order, and the sources inside each in the order
written, because the project list, and with it the manifest row order,
follows the order in which paths are first seen:

| # | Sources | sh | PowerShell |
| --- | --- | --- | --- |
| 1 | Claude Code: prompt history and `.claude.json` project keys, also in `CLAUDE_CONFIG_DIR` homes | `find_claude_code_projects` | `Find-ClaudeCodeProjects` |
| 2 | Codex (a rollout `cwd`, in `sessions` or `archived_sessions`, outside `Documents/Codex` and `.codex/worktrees`), Qwen Code, Cline CLI, Cursor chats, Continue, Goose legacy sessions (Linux and macOS, and Windows), OpenCode, Crush, Agent Zero editor (ACP) sessions | `find_cli_session_projects` | `Find-CliSessionProjects` |
| 3 | Gemini CLI projects, trusted folders and project roots; Antigravity CLI history, settings and project cache; Gemini config projects | `find_gemini_projects` | `Find-GeminiProjects` |
| 4 | Amp, Factory, Kiro, Cursor `acp-sessions`, Goose desktop recent directories, scheduled jobs (`recipe_base_dir`) and project files (`projects/*.md` `workingDirs`), Copilot CLI (sessions with an `events.jsonl`, a `cwd` outside `.copilot/`), Hermes, Letta, pi, muse, Open Interpreter (a rollout `cwd` outside `.openinterpreter/worktrees`), OpenHands, PearAI | `find_agent_session_projects` | `Find-AgentSessionProjects` |
| 5 | OpenClaw (and clawdbot), nanobot, Tabby, Twinny | `find_config_file_projects` | `Find-ConfigFileProjects` |
| 6 | VS Code-family `User` directories: `workspaceStorage` folders and the Cline, Roo Code and Kilo Code task indexes | `find_editor_projects` | `Find-EditorProjects` |

The format readers are shared by the groups: `json_vals` /
`Get-JsonValues`, `json_path_keys` / `Get-JsonKeys`, `abs_strings` /
`Get-PathStrings`, `yaml_vals` / `Get-YamlValues`, `json5_vals` /
`Get-Json5Values`, `toml_file_urls` / `Get-TomlFileUrls` and `common_dir`
/ `Get-CommonDir`. The sh groups run with `IFS` set to newline only
(`discover_projects` sets it), so their globs keep paths with spaces
whole.

After the groups, each side adds the `-p` / `-Project` directories with
their owner (`get_project_owner` / `Get-ProjectOwner`) and reduces the list
to existing directories that are not homes, each once. The sh collector
keeps only values that start with `/`, undoes `\/`, prefixes the root
(`discover_projects`), keeps the first non-empty owner per path
(`merge_project_owners`) and drops non-directories and homes
(`keep_project_dirs`). The PowerShell collector turns each value into a
local path with `ConvertTo-LocalPath` (which also takes drive-letter paths
and `file://` URIs) and deduplicates without regard to case in
`Select-UniqueProjects`. Under inventory mode no credential file is read:
the sh readers drop the files `test_inventory_secret` rejects (the
`INVENTORY_DROP_SECRETS` snippet, `eval`'d because a function cannot change
its caller's `"$@"`), and the PowerShell readers go through
`Read-DiscoverySource`, which asks `Test-InventorySecret`.

A new source goes into its group on both sides, at the same position.

## Inventory mode

`--inventory` / `-Inventory` runs the same walk and writes nothing; the
output is described in [inventory.md](inventory.md).

- sh: `set_output_names` sets `LOG` and `MANIFEST` to `/dev/null` and no
  staging directory. `collect_catalog` prints `M|agent|glob` for each
  match of an agent other than `shared` (a `shared` match still claims its
  path but is not walked), `collect_tree` prunes exclusions without a row
  and gives regular files to `INVENTORY_FILES_WORKER`, `collect_path`
  skips a matched file that `test_excluded` matches, and
  `write_inventory` pipes each user's lines through `aggregate_inventory`
  (awk, which also converts epoch seconds to dates with plain arithmetic)
  and `write_inventory_lines`. `count_inventory_homes` counts the homes
  and the unreadable ones.
- PowerShell: `Open-RunFiles` opens no writers. `Add-CatalogMatches` calls
  `Add-InventoryEvidence`, `Add-CollectedFile` calls `Add-InventoryFile`, and
  `Add-ExcludedPath` and `Write-ReadErrorRow` write no row. Each user's
  accumulator (`$script:InventoryAgents`) is kept in `$script:InventoryUsers`, and
  `Write-Inventory` prints the lines with `Write-InventoryLines`.

## Naming

sh, which has no `local`:

- Globals shared between functions (configuration, counters, output
  paths, the tables) are `UPPER_CASE`.
- A function's own temporaries are `_<prefix>_<name>`, the prefix an
  abbreviation of the function's name stated in its comment (`_cp_` for
  `collect_path`), so a callee cannot clobber its caller's variables.
- A function that assigns its result instead of printing it, to save a
  fork, sets `_ret_<what>`: `_ret_json`, `_ret_number`, `_ret_dir_error`,
  `_ret_hash`, `_ret_heredoc_text`, `_ret_volume_dir`.
- A worker program is held in `<NAME>_WORKER` and read from the
  here-document `EOF_<NAME>_WORKER`; its variables need no prefix. Other
  here-document labels name what they feed (`EOF_USER_LIST`,
  `EOF_VOLUME_LIST`, `EOF_DOCKER_DESKTOP_DIRS`, `EOF_EXTRA_PROJECTS`,
  `EOF_COMMON_HELPERS`, `EOF_COMMON_MANIFEST`, `EOF_SUMMARY` for
  `collection.json`, `EOF_USAGE` for the usage text).

PowerShell:

- Script state shared between functions is `$script:PascalCase`, written
  and read with that prefix; the parameters, the five tables, `$TOOL` and
  `$ToolVersion` are read by their plain names. Function parameters and
  locals are `$camelCase` and never an automatic variable's name (`$home`,
  `$host`, `$args`, `$input`, `$error`, `$matches`, `$pid`, `$profile`).
- Functions use approved verbs. Functions that set up or change run state
  are named with `Initialize`, `Open`, `Close`, `Switch` and `Clear`, not
  `New`, `Set`, `Remove`, `Start`, `Stop` or `Update`: PSScriptAnalyzer's
  default rule `PSUseShouldProcessForStateChangingFunctions` requires
  `ShouldProcess` support (the `-WhatIf` and `-Confirm` plumbing) for
  those verbs.

Both: a name says what the value holds, in words, and is at least three
characters after its temporaries prefix, except a loop index (`i`, `j`,
`k`) whose scope is a few lines; variables that fill a manifest field are
named like the field.

## Function pairs

A sh function and a PowerShell function that do the same step have
matching names and one row here. Worker programs are listed beside the
PowerShell functions that do their job.

| Step | sh | PowerShell |
| --- | --- | --- |
| Append a line to the log, and to stderr unless quiet | `log_line` | `Write-CollectorLog` |
| Record a note for `collection.json` | `add_note` | `Add-Note` |
| Compile the secret globs into one pattern (PowerShell: the exclusions too) | `compile_secret_pattern` | `Initialize-TableRegexes` |
| JSON-escape a string | `json_escape`, `json_escape_var` | `ConvertTo-JsonEscaped` |
| SHA-256 of one file | `hash_file` | `Get-FileSha256` |
| Write one manifest row | `write_manifest_row` | `Write-ManifestRow` |
| Why a directory cannot be walked | `get_dir_read_error`, `get_dir_read_error_var` | `Get-DirReadError` |
| Write an `error_read` row | `write_read_error_row` | `Write-ReadErrorRow` |
| Stage one file: metadata, symlink or copy, hash, row | `STAGE_FILES_WORKER` | `Add-CollectedFile` |
| Classify a path: file, symlink or special file (`socket`, `fifo`, `device`, `other`) | the `type_N` tests in `plan_chunk` | `Get-ManifestType` |
| Record an excluded path | `EXCLUDED_PATHS_WORKER` | `Add-ExcludedPath` |
| Count one file under inventory | `INVENTORY_FILES_WORKER` | `Add-InventoryFile` |
| Expand one glob under a base | `expand_glob` | `Expand-Glob` |
| Expand a table against a base: matches, case variants, claims | `expand_catalog` | `Expand-Catalog` |
| Collect every match of a table under a base | `collect_catalog` | `Add-CatalogMatches` |
| Collect one matched path | `collect_path` | `Add-CollectedPath` |
| Walk a matched directory | `collect_tree` | `Add-CollectedTree` |
| List candidate homes, duplicates included | `enumerate_users` | `Get-CandidateHomes` |
| List the homes to collect | `get_user_homes` | `Get-UserHomes` |
| Collect every home (main step) | `collect_user_homes` | `Add-UserHomes` |
| Match a volume name to `DOCKER_VOLUMES` | `get_docker_volume_rule` | `Get-DockerVolumeRule` |
| Check a volumes directory on the way down | `test_volume_root` | `Test-VolumeRoot` |
| Collect every volume in one volumes directory | `collect_volume_dir` | `Add-VolumeDir` |
| Collect or record one volume | `collect_volume` | `Add-Volume` |
| Every Docker and Podman root (main step) | `collect_docker_volumes` | `Add-DockerVolumes` |
| Is a discovery source a credential file under inventory | `test_inventory_secret` | `Test-InventorySecret` |
| JSON string values of a key | `json_vals` | `Get-JsonValues` |
| JSON keys that are absolute paths | `json_path_keys` | `Get-JsonKeys` |
| Quoted absolute paths and `file://` URIs | `abs_strings` | `Get-PathStrings` |
| Top-level YAML values of a key | `yaml_vals` | `Get-YamlValues` |
| JSON5 string values of a key | `json5_vals` | `Get-Json5Values` |
| TOML `key = "file://..."` values | `toml_file_urls` | `Get-TomlFileUrls` |
| Deepest directory shared by file paths | `common_dir` | `Get-CommonDir` |
| Owner of a `-p` directory | `get_project_owner` | `Get-ProjectOwner` |
| Discovery group 1, Claude Code | `find_claude_code_projects` | `Find-ClaudeCodeProjects` |
| Discovery group 2, Codex through Agent Zero | `find_cli_session_projects` | `Find-CliSessionProjects` |
| Discovery group 3, Gemini CLI and Antigravity | `find_gemini_projects` | `Find-GeminiProjects` |
| Discovery group 4, Amp through PearAI | `find_agent_session_projects` | `Find-AgentSessionProjects` |
| Discovery group 4, Goose's recent directories, scheduled jobs and project files | `find_goose_projects` | `Find-GooseProjects` |
| Discovery group 5, configuration files | `find_config_file_projects` | `Find-ConfigFileProjects` |
| Discovery group 6, editor `User` directories | `find_editor_projects` | `Find-EditorProjects` |
| Discover every project | `discover_projects` | `Find-Projects` |
| Collect every project (main step; the sh one also runs discovery) | `collect_projects` | `Add-Projects` |
| Write the inventory lines (main step) | `write_inventory` | `Write-Inventory` |
| Format one user's inventory lines | `write_inventory_lines` | `Write-InventoryLines` |
| Write `collection.json` and stage a copy | `write_summary` | `Write-Summary` |
| Write the archive, with fallbacks (main step) | `write_archive` | `Write-Archive` |
| Hash the archive, write `.sha256`, log `done` | `hash_archive` | `Write-ArchiveHash` |
| Remove the staging directory | `cleanup` | `Clear-Staging` |
| Print the stdout summary (main step) | `print_result` | `Write-Result` |
| Bytes to `20.9 MiB` | `format_human_size` | `Format-HumanSize` |
| Print the tables for `--list` | `print_catalog` | `Write-CatalogList` |
| Resolve the extra project arguments | `resolve_extra_projects` | `Resolve-ProjectArgs` |
| Resolve the root and set the mode | `resolve_root` | `Resolve-RootArg` |
| Name the run and its output files | `set_output_names` | `Initialize-RunNames` |
| Create the staging directory, log and manifest | `create_output_files` | `Open-RunFiles` |
| Log the start lines | `log_start` | `Write-StartLog` |

### sh only

| Function | Section | What it does |
| --- | --- | --- |
| `heredoc_var` | Shared helpers | Reads a here-document into `_ret_heredoc_text` |
| `utc_timestamp` | Shared helpers | The current UTC time, ISO 8601 to the second |
| `stat_file`, `stat_files` | Shared helpers | `lstat` metadata of one file, or of many in one call |
| `hash_files`, `hash_match` | Shared helpers | Hashes many files in one call; matches one output line to a file |
| `to_number`, `to_number_var` | Shared helpers | A non-negative integer or 0 |
| `file_id` | Shared helpers | `device:inode` of a path, for case-variant folding |
| `define_secret_test`, `is_secret` | Shared helpers | Defines `is_secret` from `SECRET_PATTERN` with one `eval`; whether a path relative to the base matches a secret glob |
| `lstat_chunk`, `plan_chunk`, `copy_chunk`, `copy_run`, `hash_chunk`, `write_chunk_rows` | Staging files | The passes of `STAGE_FILES_WORKER`, defined inside it |
| `UNREADABLE_DIRS_WORKER` | Staging files | `error_read` rows for directories inside a walk; PowerShell does this in `Add-CollectedTree` |
| `print_file_rows` | Staging files | Inside `INVENTORY_FILES_WORKER`: the `F\|` lines from one `stat` call |
| `classify_case_matches`, `fold_case_matches` | Catalog walk | Case-variant folding; PowerShell uses `Get-OnDiskName` |
| `test_excluded` | Catalog walk | Whether a match that is not a directory is excluded; PowerShell tests `$script:ExcludeRegex` in `Add-CollectedPath` |
| `filter_users` | Users | Drops system and shared homes, applies `-u`, keeps each home once; PowerShell does this in `Get-UserHomes` |
| `find_errors_since` | Docker and Podman volumes | `find:` log lines since a given line |
| `session_cwds` | Project discovery | `cwd` values in rollout files found with `find` |
| `merge_project_owners`, `keep_project_dirs` | Project discovery | Deduplicates projects; PowerShell uses `Select-UniqueProjects` |
| `report_missing_projects` | Project discovery | Log line and `error_read` row for a `-p` that is not a directory; PowerShell does this in `Add-Projects` |
| `aggregate_inventory` | Inventory output | Sums `M\|` and `F\|` lines per agent; PowerShell accumulates with `Get-InventoryEntry`, `Add-InventoryEvidence`, `Add-InventoryFile` |
| `count_inventory_homes` | Inventory output | Homes scanned and unreadable; PowerShell does this in `Write-Inventory` |
| `count_status`, `list_active_users`, `sum_collected_bytes` | Summary and archive | Counts read back from the manifest; PowerShell counts in `Write-ManifestRow` |
| `json_list`, `json_bool` | Summary and archive | JSON arrays and booleans for `collection.json` |
| `tar_gzip` | Summary and archive | The `tar \| gzip` fallback |
| `usage`, `need_value`, `parse_options` | Main program | Help text and option parsing; PowerShell has `param()` |
| `set_max_size` | Main program | Validates `--max-file-size` |
| `check_prerequisites` | Main program | Output directory, `tar` and `find` checks; PowerShell has `Initialize-OutputDir` |
| `detect_capabilities` | Main program | `HASH_TOOL`, `STAT_MODE`, `WORKER_SH` |

### PowerShell only

| Function | Section | What it does |
| --- | --- | --- |
| `Get-TableLines` | Shared helpers | A table's lines without blanks and `#` comments |
| `Convert-GlobToRegex`, `ConvertTo-TableRegex` | Shared helpers | Glob to anchored regex; one regex per table |
| `ConvertTo-ExtendedPath`, `ConvertFrom-ExtendedPath`, `ConvertFrom-ExtendedText` | Shared helpers | Extended-length paths and back |
| `Test-PathQuiet`, `Test-IsLink`, `Test-DirReadable` | Shared helpers | File-system probes that never throw |
| `Join-PathSafe`, `Get-RelPath` | Shared helpers | Path joining and relative paths |
| `Get-Epoch`, `Get-OwnerName`, `Get-DirSize`, `Get-LinkTarget` | Shared helpers | Metadata for rows |
| `Get-ArchivePath` | Staging files | `fs/<original path>` for a source path |
| `Copy-FileShared` | Staging files | Copies with shared access and hashes the copy |
| `Get-LinkApi`, `Invoke-Mklink`, `Invoke-CreateSymlink`, `Add-StagedLink` | Staging files | Recreates symlinks in the staging tree |
| `Get-ClaimTable`, `Get-OnDiskName`, `Write-GlobErrorRows` | Catalog walk | Claim table; on-disk spelling; rows for unlistable glob directories |
| `Get-DeniedHomes` | Users | Homes listed by their parent but not reachable |
| `Write-UnmatchedVolumeRow`, `Find-DockerDesktop` | Docker and Podman volumes | The `skipped_unmatched_volume` row; Docker Desktop notes |
| `ConvertTo-LocalPath`, `Read-DiscoverySource`, `Get-RegexCaptures`, `Get-FileTexts`, `Select-UniqueProjects` | Project discovery | Value to local path; reading sources; deduplication |
| `Get-InventoryEntry`, `Add-InventoryEvidence`, `Format-InventoryTime` | Inventory output | Inventory accumulators and dates |
| `Initialize-Summary`, `Add-LinkNotes` | Summary and archive | Builds the `collection.json` content |
| `Close-RunFiles`, `Add-LogLine` | Summary and archive | Closes the writers; log lines after that |
| `Get-StageEntries`, `Get-UnixTime`, `Get-TarHeader`, `Get-PaxRecord`, `Get-TarAsciiName`, `Write-TarMember`, `Get-StagedLinkTargets`, `Get-StagedLinkTarget`, `Write-TarEntries`, `Write-TarGz` | Summary and archive | The PowerShell tar writer |
| `Write-StageZip` | Summary and archive | The zip archiver |
| `Invoke-Archiver`, `Switch-Archiver`, `Format-ExitCode`, `Clear-PartialArchive` | Summary and archive | Running and switching archivers |
| `Clear-StagedLinks`, `Clear-StagedLinksForZip`, `Clear-LinkArchivePaths` | Summary and archive | Removing staged links and updating their rows |
| `Get-NonEmptyLines` | Main program | `-List` lines |
| `Initialize-OutputDir`, `Initialize-Archivers`, `Test-Administrator` | Main program | Output directory check, archiver chain, Administrator test |
