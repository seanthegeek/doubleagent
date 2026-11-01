#!/usr/bin/env python3
"""Build the synthetic collection of host `wsdev14` that the output examples are a run on.

The agent files are the analyzer's test fixtures: each user's home is written by the
`HOME_BUILDERS` entries of `analyzer/tests/fixtures/`, then its `EXAMPLE_BUILDERS` entries, whose
module is one of that user's fixture modules in `USER_AGENTS`, in their registry order, and
`/srv/proj` by `build_git_repo` and `claude_project_local_settings`. A change to a builder
therefore shows in the examples, and `tools/check_fixture_schema.py`, which runs `build_home`,
covers the records of the `HOME_BUILDERS` entries. Amp has no parser and so no fixture; its one
settings file is written here.

The rest is this module's: the walk that turns the built trees into manifest rows the way the sh
collector would (catalog entries in order, `EXCLUDES` as `skipped_excluded` rows, `SECRET_GLOBS`
as `secret: true`), `collection.json` and `collector.log` in the shape
`collectors/docs/output.md` documents, and the damage the fixtures do not hold: one manifest row
is cut short (`CUT_GLOB`), one collected file is left out of `fs/` (`MISSING_GLOB`), and two
excluded directories carry the sizes of a used install (`SKIPPED_SIZES`). A file a builder leaves
at the time of the build is given `DEFAULT_MTIME`, so every run writes the same collection.

    python3 analyzer/tools/example_collection.py <empty directory>

writes the collection to `<empty directory>/wsdev14` and prints that path. Exit status: 0
when it was written, 2 unless the one argument names an empty directory; a failed write raises.
"""

from __future__ import annotations

import fnmatch
import hashlib
import importlib
import json
import os
import sys
import tempfile
import time
from collections.abc import Iterator
from datetime import datetime, timezone
from pathlib import Path
from typing import NamedTuple

TOOLS = Path(__file__).resolve().parent
ANALYZER = TOOLS.parent
# The examples are built from the test fixtures on purpose, so they cannot drift from the records
# the parsers are tested against; `analyzer/tests` holds the `fixtures` package.
for directory in (ANALYZER, ANALYZER / "tests"):
    if str(directory) not in sys.path:
        sys.path.insert(0, str(directory))

# Imported after the analyzer and its tests join the path, so the tool runs from any directory.
from doubleagent import catalog as catalog_tables  # noqa: E402

fixtures = importlib.import_module("fixtures")

HOST = "wsdev14"
COLLECTOR_VERSION = "1.19.0"
UIDS = {"alice": 1000, "bob": 1001}
# The developers' shared group that owns `/srv/proj`, so both users write to it.
PROJECT_GID = 1002
PROJECT = "/srv/proj"
HOMES = {"alice": "/home/alice", "bob": "/home/bob"}
# The fixture modules whose home builders write each user's home. Codex CLI is alice's because
# its state database names rollouts under `/home/alice`.
USER_AGENTS = {
    "alice": ("claude_code", "codex_cli", "cursor", "aider"),
    "bob": ("gemini_cli", "ollama"),
}
AMP_SETTINGS = """\
{
  "amp.notifications.enabled": true,
  "amp.showCosts": false,
  "amp.updates.mode": "warn"
}
"""
# Excluded directories created empty, with the size `du -sk` gives on a used install, so the
# examples show `skipped_excluded` rows of a realistic size.
SKIPPED_SIZES = {
    ("alice", ".claude/cache"): 3168 * 1024,
    ("bob", ".ollama/models/blobs"): 4573340 * 1024,
}
# The collected file `fs/` lacks, as if lost after the collection, and the file whose manifest row
# is cut short, so the examples show the `problem:` and `note:` lines; each glob is matched
# against `<user>/<path from the home>` and must match exactly one built file.
MISSING_GLOB = "bob/.gemini/tmp/*/chats/session-*.json"
CUT_GLOB = "bob/.ollama/models/manifests/*"
DAY = "2026-10-15"
STARTED = "16:56:10"
FINISHED = "16:56:28"
# The modification time of a file whose builder sets none, an hour before the collection.
DEFAULT_MTIME = "15:56:10"
# The accounts `getent passwd` lists on this host whose home exists, in passwd order: the
# collector scans the system accounts' homes too and finds nothing in them.
SCANNED_HOMES = [
    ("root", "/root"),
    ("games", "/usr/games"),
    ("man", "/var/cache/man"),
    ("mail", "/var/mail"),
    ("backup", "/var/backups"),
    ("alice", HOMES["alice"]),
    ("bob", HOMES["bob"]),
]
CATALOG = catalog_tables.load()


def epoch(clock: str) -> int:
    """Return `clock` (`HH:MM:SS`) on the example's day in epoch seconds."""
    moment = datetime.strptime("%s %s" % (DAY, clock), "%Y-%m-%d %H:%M:%S")
    return int(moment.replace(tzinfo=timezone.utc).timestamp())


class FileEntry(NamedTuple):
    """One manifest row: a collected file with its bytes, or an excluded directory without."""

    user: str
    home: str
    agent: str
    rel: str
    data: bytes
    flagged: bool
    mtime: int
    uid: int
    excluded: bool = False
    size: int = 0


def build_trees(work: Path) -> dict[str, Path]:
    """Write every user's home and the project under `work`; return each base by its shown path.

    A builder's directory in its registry is relative to the home, as `build_home` runs it.
    """
    modules = {
        name: importlib.import_module("fixtures." + name)
        for names in USER_AGENTS.values()
        for name in names
    }
    bases: dict[str, Path] = {}
    for user, names in USER_AGENTS.items():
        home = work / user
        home.mkdir(parents=True)
        for builder, directory in fixtures.HOME_BUILDERS + fixtures.EXAMPLE_BUILDERS:
            if any(builder.__module__ == modules[name].__name__ for name in names):
                builder(home / directory)
        bases[HOMES[user]] = home
    amp = work / "alice/.config/amp/settings.json"
    amp.parent.mkdir(parents=True, exist_ok=True)
    amp.write_text(AMP_SETTINGS, encoding="utf-8")
    for user, rel in SKIPPED_SIZES:
        (work / user / rel).mkdir(parents=True, exist_ok=True)
    project = work / "project"
    fixtures.build_git_repo(project)
    settings = project / ".claude/settings.local.json"
    settings.parent.mkdir(parents=True)
    settings.write_text(json.dumps(fixtures.claude_project_local_settings()), encoding="utf-8")
    bases[PROJECT] = project
    return bases


def _matches(rel: str, globs: list[str]) -> bool:
    # Exclusion and secret globs match the whole path from the base, `*` crossing `/`.
    return any(fnmatch.fnmatchcase(rel, glob) for glob in globs)


def _du_size(path: Path) -> int:
    total = sum(item.lstat().st_size for item in path.rglob("*") if item.is_file())
    return -(-total // 1024) * 1024


def walk_base(
    user: str, shown: str, disk: Path, entries: list, built_after: float
) -> Iterator[tuple[str, FileEntry | None]]:
    """Yield (log line, row) for one home or the project, in the collector's order.

    Each catalog entry that matches gives its log line with no row, then its files and
    excluded directories, sorted; a file a nested entry already gave is not repeated. `agent` is
    the most specific entry's, `flagged` a `SECRET_GLOBS` match, and a file modified at or after
    `built_after` gets `DEFAULT_MTIME`.
    """
    project = shown == PROJECT
    tables = catalog_tables.Catalog(entries=entries)
    seen: set[str] = set()
    for entry, hit in tables.matches_in_home(disk):
        rel = hit.relative_to(disk).as_posix()
        yield "  [%s] %s/%s" % (entry.agent, shown, rel), None
        stack = [hit]
        while stack:
            path = stack.pop()
            rel = path.relative_to(disk).as_posix()
            if rel in seen:
                continue
            seen.add(rel)
            if path.is_symlink():
                raise ValueError("a fixture builder wrote a symlink, %s; add its row form" % rel)
            agent = (
                CATALOG.project_agent_for(rel) if project else CATALOG.claims_home_path(rel)
            ) or entry.agent
            owner = "alice" if project else user
            if path.is_dir() and _matches(rel, CATALOG.excludes):
                size = SKIPPED_SIZES.get((user, rel), _du_size(path))
                row = FileEntry(owner, shown, agent, rel, b"", False, 0, 0, True, size)
                yield "", row
            elif path.is_dir():
                stack.extend(sorted(path.iterdir(), reverse=True))
            else:
                mtime = path.lstat().st_mtime
                stamp = epoch(DEFAULT_MTIME) if mtime >= built_after else int(mtime)
                data = path.read_bytes()
                flagged = _matches(rel, CATALOG.secrets)
                row = FileEntry(owner, shown, agent, rel, data, flagged, stamp, UIDS[owner])
                yield "", row


def files(work: Path) -> tuple[list[FileEntry], list[str]]:
    """Build the trees under `work`; return every manifest row in order and the log's lines.

    Homes come in `SCANNED_HOMES` order, then the project, as the collector writes them.
    """
    built_after = time.time() - 1
    bases = build_trees(work)
    rows: list[FileEntry] = []
    log = [
        "%s %s starting on %s (Linux 6.8.0-45-generic) mode=live root=/ uid=0"
        % ("collect-agent-artifacts", COLLECTOR_VERSION, HOST),
        "hash=sha256sum stat=gnu max_file_size=256MB full=0 no_secrets=0 no_docker=0"
        " worker_sh=dash",
    ]
    targets = [(user, home, CATALOG.entries) for user, home in SCANNED_HOMES if home in bases]
    targets.append(("alice", PROJECT, CATALOG.project_entries))
    for user, shown, entries in targets:
        log.append(
            "Project [%s] %s" % (user, shown)
            if shown == PROJECT
            else "User %s (%s)" % (user, shown)
        )
        for line, row in walk_base(user, shown, bases[shown], entries, built_after):
            if line:
                log.append(line)
            if row is not None:
                rows.append(row)
    log.append("Archiving")
    return rows, log


def manifest_row(entry: FileEntry) -> dict:
    """Return the manifest row of `entry`, every field in the documented order."""
    path = entry.home + "/" + entry.rel
    if entry.excluded:
        # The sh collector sizes an excluded directory with du and records no times or owner.
        row: dict = {"user": entry.user, "home": entry.home, "agent": entry.agent, "path": path}
        row.update(archive_path="", type="dir", size=entry.size)
        row.update(mtime=0, atime=0, ctime=0, btime=0, uid=0, gid=0, mode="0", sha256="")
        row.update(secret=False, status="skipped_excluded", target="", error="")
        return row
    return {
        "user": entry.user,
        "home": entry.home,
        "agent": entry.agent,
        "path": path,
        "archive_path": "fs" + path,
        "type": "file",
        "size": len(entry.data),
        "mtime": entry.mtime,
        "atime": entry.mtime,
        "ctime": entry.mtime,
        "btime": entry.mtime,
        "uid": entry.uid,
        "gid": PROJECT_GID if entry.home == PROJECT else entry.uid,
        # Credential files are private to their user; the rest follow the users' umask of 002.
        "mode": "600" if entry.flagged else "664",
        "sha256": hashlib.sha256(entry.data).hexdigest(),
        "secret": entry.flagged,
        "status": "collected",
        "target": "",
        "error": "",
    }


def summary(rows: list[dict]) -> dict:
    """Return `collection.json` for the manifest `rows`, in the sh collector's key order."""
    statuses = (
        "collected",
        "symlink",
        "skipped_excluded",
        "skipped_size",
        "skipped_secret",
        "skipped_special",
        "error_copy",
        "skipped_unmatched_volume",
        "error_read",
    )
    counts: dict = {status: sum(row["status"] == status for row in rows) for status in statuses}
    counts["collected_bytes"] = sum(row["size"] for row in rows if row["status"] == "collected")
    options = {"full": False, "no_secrets": False, "max_file_size_bytes": 268435456}
    options.update(users_filter="", no_docker=False, no_projects=False, projects=[])
    capabilities = {"hash_tool": "sha256sum", "stat_mode": "gnu", "worker_shell": "dash"}
    capabilities["archiver"] = "tar -z"
    users = [name for name, home in SCANNED_HOMES if any(row["home"] == home for row in rows)]
    return {
        "tool": "collect-agent-artifacts",
        "version": COLLECTOR_VERSION,
        "hostname": HOST,
        "mode": "live",
        "root": "/",
        "platform": "Linux 6.8.0-45-generic x86_64",
        "run_as_uid": "0",
        "started": "%sT%sZ" % (DAY, STARTED),
        "finished": "%sT%sZ" % (DAY, FINISHED),
        "options": options,
        "capabilities": capabilities,
        "users": [name for name, _ in SCANNED_HOMES],
        "homes": [home for _, home in SCANNED_HOMES],
        "users_with_artifacts": users,
        "projects": [PROJECT],
        "counts": counts,
        "docker": {
            "volumes_found": 0,
            "volumes_collected": 0,
            "unreadable": 0,
            "docker_desktop": False,
        },
        "notes": [],
        "archive": "%s_%sT%sZ_agent-artifacts.tar.gz"
        % (HOST, DAY.replace("-", ""), STARTED.replace(":", "")),
    }


def _one_path(rows: list[FileEntry], glob: str) -> str:
    """Return the shown path of the one collected row whose `<user>/<path>` matches `glob`."""
    found = [
        row.home + "/" + row.rel
        for row in rows
        if not row.excluded and fnmatch.fnmatchcase(row.user + "/" + row.rel, glob)
    ]
    if len(found) != 1:
        raise ValueError("%s must match one fixture file, matched %d" % (glob, len(found)))
    return found[0]


def build_collection(root: Path) -> Path:
    """Write the `wsdev14` collection under `root` and return its directory.

    Args:
        root: An empty directory; the collection is written to `root/wsdev14`.

    Returns:
        The collection directory: `collection.json`, `manifest.jsonl`, `collector.log` and `fs/`.
    """
    collection = root / HOST
    (collection / "fs").mkdir(parents=True)
    with tempfile.TemporaryDirectory(prefix="doubleagent-example-") as work:
        entries, log = files(Path(work))
    missing = _one_path(entries, MISSING_GLOB)
    cut = _one_path(entries, CUT_GLOB)
    rows: list[dict] = []
    lines: list[str] = []
    for entry in entries:
        row = manifest_row(entry)
        rows.append(row)
        line = json.dumps(row, separators=(",", ":"))
        if row["path"] == cut:
            line = line[: line.index(cut) + len(entry.home) + 20]
        lines.append(line)
        if entry.excluded or row["path"] == missing:
            continue
        disk = collection / row["archive_path"]
        disk.parent.mkdir(parents=True, exist_ok=True)
        disk.write_bytes(entry.data)
        os.utime(disk, (row["mtime"], row["mtime"]))
    (collection / "manifest.jsonl").write_text("\n".join(lines) + "\n", encoding="utf-8")
    stamp = "%sT%sZ " % (DAY, STARTED)
    text = "".join(stamp + line + "\n" for line in log[:-1])
    text += "%sT%sZ %s\n" % (DAY, FINISHED, log[-1])
    (collection / "collector.log").write_text(text, encoding="utf-8")
    text = json.dumps(summary(rows), indent=2) + "\n"
    (collection / "collection.json").write_text(text, encoding="utf-8")
    return collection


def main(argv: list[str] | None = None) -> int:
    """Write the collection into the directory named by the one argument; return the exit status."""
    arguments = sys.argv[1:] if argv is None else argv
    if len(arguments) != 1:
        print("usage: example_collection.py <empty directory>", file=sys.stderr)
        return 2
    root = Path(arguments[0])
    if not root.is_dir():
        print("error: %s is not a directory" % root, file=sys.stderr)
        return 2
    if any(root.iterdir()):
        print("error: %s is not empty" % root, file=sys.stderr)
        return 2
    print(build_collection(root))
    return 0


if __name__ == "__main__":
    sys.exit(main())
