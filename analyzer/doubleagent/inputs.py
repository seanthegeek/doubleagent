"""Open whatever the analyst points at and turn it into attributed artifacts.

Three input shapes are accepted:

* a collector archive (`*_agent-artifacts.tar.gz` or `.zip`), extracted to a
  work directory first;
* an extracted collection: a directory holding `manifest.jsonl`,
  `collection.json` and `fs/`;
* a loose directory: anything else, such as a copied home directory, a
  mounted image, another collector's output, or a single agent directory
  like `.claude` copied on its own.

With a manifest, attribution (host, user, home, agent) is read from it. Without
one, homes are discovered by matching the catalog against the tree and the
user is inferred from the path, which the output marks by leaving `host`
empty unless `--host` is given.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import stat
import tarfile
import tempfile
import zipfile
from collections.abc import Iterator
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath

from .catalog import Catalog
from .errors import InputError
from .textlines import JSON_SPACE

PARSE_STATUSES = ("collected",)

# The agent of a project file no single agent owns (AGENTS.md), and of
# every project-catalog file in collections before collector 1.12.0. Such a
# file is offered to every parser.
PROJECT_AGENT = "project"

# Parent directories whose children are user homes, as the collectors enumerate
# them. Matched against the path of a discovered home relative to the root.
HOME_PARENTS = ("home", "Users", "usr/home", "export/home")
ROOT_HOMES = ("root", "var/root")


@dataclass
class Home:
    """A base the artifacts are relative to: a user's home, a volume or a project."""

    disk_path: Path  # where the home is on the analyst's disk
    original: str  # path on the source host, best effort
    user: str
    host: str = ""
    inferred: bool = False  # True when user/original came from the path, not a manifest
    # True when this base is a project directory the collector collected from
    # (collection.json `projects`, or a manifest row with agent `project`),
    # not a user's home. Parsers that scope a file to its project read this,
    # never the artifact's agent.
    project: bool = False


@dataclass
class Artifact:
    """One attributed file of the input, with its manifest row when there is one."""

    disk_path: Path
    original: str  # path on the source host
    rel: str  # path relative to the home, posix
    agent: str
    home: Home
    manifest: dict | None = None


@dataclass
class Collection:
    """The opened input: its kind, homes, artifacts, notes and problems."""

    source: Path
    kind: str  # archive | collected | loose
    root: Path  # directory that holds fs/ (collected) or the tree itself (loose)
    host: str = ""
    manifest: list[dict] | None = None
    summary: dict | None = None
    homes: list[Home] = field(default_factory=list)
    artifacts: list[Artifact] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)
    # "<path>: <reason>" for each directory or file inside the input that
    # could not be read while the input was opened.
    problems: list[str] = field(default_factory=list)
    _extracted_dir: str | None = None

    def cleanup(self) -> None:
        """Remove the directory an archive was extracted to, if any."""
        if self._extracted_dir and os.path.isdir(self._extracted_dir):
            shutil.rmtree(self._extracted_dir, ignore_errors=True)
            self._extracted_dir = None

    def artifacts_for(self, agent: str) -> list[Artifact]:
        """Return the artifacts attributed to one agent."""
        return [artifact for artifact in self.artifacts if artifact.agent == agent]

    def agents(self) -> list[str]:
        """Return the agents of the artifacts, in the order first met."""
        seen: dict[str, None] = {}
        for artifact in self.artifacts:
            seen.setdefault(artifact.agent)
        return list(seen)


def error_reason(error: BaseException) -> str:
    """Return an exception's reason without the path it names.

    An OSError gives its OS message lower-cased (`permission denied`), any
    other exception its own text.
    """
    if isinstance(error, OSError) and error.strerror:
        return error.strerror.lower()
    return str(error)


def describe_error(path, error: BaseException) -> str:
    """Return `<path>: <reason>` for a problem line or an error message.

    The path is the file the OS reported when it names one, otherwise `path`.
    """
    name = error.filename if isinstance(error, OSError) and error.filename else path
    return "%s: %s" % (os.fsdecode(name), error_reason(error))


def open_input(
    path: Path, catalog: Catalog, work_dir: Path | None = None, host: str = ""
) -> Collection:
    """Open an archive, an extracted collection or a loose directory.

    Unreadable directories and files inside a loose tree are not errors; they
    are recorded in `Collection.problems`.

    Raises:
        OSError: The input, an archive member or the manifest cannot be read.
        InputError: The input is not a file or directory, or not an archive it can extract.
    """
    path = Path(path)
    # stat, not exists(): exists() hides the reason, and from Python 3.14
    # returns False for a permission error.
    file_stat = path.stat()
    if stat.S_ISDIR(file_stat.st_mode):
        os.listdir(path)  # an unlistable input directory is an error, not an empty tree
        if (path / "manifest.jsonl").is_file() and (path / "fs").is_dir():
            collection = Collection(source=path, kind="collected", root=path)
        else:
            collection = Collection(source=path, kind="loose", root=path)
    elif stat.S_ISREG(file_stat.st_mode):
        collection = _open_archive(path, work_dir)
    else:
        raise InputError("%s is neither a regular file nor a directory" % path)
    if host:
        collection.host = host
    try:
        if collection.kind in ("archive", "collected"):
            _load_collected(collection, catalog)
        else:
            _discover_loose(collection, catalog)
    except BaseException:
        collection.cleanup()
        raise
    return collection


# ---- archives -----------------------------------------------------------------


def _open_archive(path: Path, work_dir: Path | None) -> Collection:
    # Opened once up front so an unreadable archive fails with its reason
    # before a work directory is made; zipfile.is_zipfile hides OSError.
    with open(path, "rb"):
        pass
    if work_dir:
        work_dir.mkdir(parents=True, exist_ok=True)
        dest = Path(tempfile.mkdtemp(prefix="cac-", dir=str(work_dir)))
    else:
        dest = Path(tempfile.mkdtemp(prefix="cac-analyzer-"))
    collection = Collection(source=path, kind="archive", root=dest)
    collection._extracted_dir = str(dest)
    try:
        sidecar = Path(str(path) + ".sha256")
        if sidecar.is_file():
            collection.notes.append(_verify_sidecar(path, sidecar))
        extract_archive(path, dest)
    except BaseException:
        collection.cleanup()
        raise
    # An archive produced by the collector unpacks to manifest.jsonl + fs/ at
    # the top; anything else is treated as a loose tree.
    if not ((dest / "manifest.jsonl").is_file() and (dest / "fs").is_dir()):
        collection.kind = "loose"
    return collection


def extract_archive(path: Path, dest: Path) -> None:
    """Extract a tar or zip archive into `dest` the way the analyzer opens an archive input.

    No link or device member is created, and a member with an absolute path, a `..` component or a
    drive letter is refused or dropped, so nothing is written outside `dest`.

    Raises:
        OSError: The archive cannot be read or a member cannot be written.
        InputError: The file is neither a tar nor a zip archive, or cannot be extracted.
    """
    try:
        if zipfile.is_zipfile(path):
            _extract_zip(path, dest)
        elif tarfile.is_tarfile(path):
            _extract_tar(path, dest)
        else:
            raise InputError("%s is neither a tar nor a zip archive" % path)
    except (tarfile.TarError, zipfile.BadZipFile, EOFError) as error:
        raise InputError("%s: cannot extract: %s" % (path, error)) from None


def _verify_sidecar(archive: Path, sidecar: Path) -> str:
    try:
        text = sidecar.read_text(encoding="utf-8", errors="replace")
    except OSError as error:
        return "WARNING archive sha256 not checked: %s" % describe_error(sidecar, error)
    # `<hash>  <name>`: the hash is the first run of characters up to ASCII whitespace.
    fields = re.findall(r"[^ \t\r\n\f\v]+", text)
    expected = fields[0].lower() if fields else ""
    digest = hashlib.sha256()
    with open(archive, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    actual = digest.hexdigest()
    if actual == expected:
        return "archive sha256 verified against %s" % sidecar.name
    return "WARNING archive sha256 %s does not match %s (%s)" % (actual, sidecar.name, expected)


def _safe_member(dest: Path, name: str) -> bool:
    member_path = PurePosixPath(name)
    return (
        not member_path.is_absolute()
        and ".." not in member_path.parts
        and not re.match(r"^[A-Za-z]:", name)
    )


def _skip_links(member, dest):
    """Pass regular files and directories through the data filter and drop links.

    The collector recreates symlinks with their absolute targets, which the
    data filter would reject; the targets are in the manifest `target` field.
    """
    if member.issym() or member.islnk() or member.isdev():
        return None
    return tarfile.data_filter(member, dest)


def _extract_tar(path: Path, dest: Path) -> None:
    with tarfile.open(path, "r:*") as archive:
        if hasattr(tarfile, "data_filter"):
            archive.extractall(dest, filter=_skip_links)
            return
        members = [
            member
            for member in archive.getmembers()
            if _safe_member(dest, member.name)
            and not (member.issym() or member.islnk() or member.isdev())
        ]
        archive.extractall(dest, members=members)


def _extract_zip(path: Path, dest: Path) -> None:
    with zipfile.ZipFile(path) as archive:
        for info in archive.infolist():
            name = info.filename.replace("\\", "/")
            if not _safe_member(dest, name):
                continue
            target = dest / name
            if name.endswith("/"):
                target.mkdir(parents=True, exist_ok=True)
                continue
            target.parent.mkdir(parents=True, exist_ok=True)
            with archive.open(info) as source, open(target, "wb") as out:
                shutil.copyfileobj(source, out)


# ---- collected layout (manifest present) --------------------------------------


def _load_collected(collection: Collection, catalog: Catalog) -> None:
    root = collection.root
    summary_path = root / "collection.json"
    if summary_path.is_file():
        try:
            collection.summary = json.loads(summary_path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as error:
            collection.notes.append("collection.json unreadable: %s" % error_reason(error))
    if not collection.host and collection.summary:
        collection.host = str(collection.summary.get("hostname") or "")
    rows: list[dict] = []
    bad = 0
    # JSONL lines end at LF; only JSON whitespace pads a record.
    with open(root / "manifest.jsonl", encoding="utf-8", errors="replace", newline="\n") as handle:
        for line in handle:
            line = line.strip(JSON_SPACE)
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except ValueError:
                bad += 1
    if bad:
        collection.notes.append("%d manifest rows did not parse" % bad)
    collection.manifest = rows
    summary_projects = (collection.summary or {}).get("projects")
    projects = {
        str(project)
        for project in (summary_projects if isinstance(summary_projects, list) else [])
        if project
    }
    homes: dict[tuple[str, str], Home] = {}
    for row in rows:
        if row.get("status") not in PARSE_STATUSES or row.get("type") != "file":
            continue
        user = str(row.get("user") or "")
        home_original = str(row.get("home") or "")
        key = (user, home_original)
        home = homes.get(key)
        if home is None:
            home = Home(
                disk_path=root / _archive_dir_for_home(row),
                original=home_original,
                user=user,
                host=collection.host,
                project=home_original in projects,
            )
            homes[key] = home
        original = str(row.get("path") or "")
        archive_path = str(row.get("archive_path") or "")
        rel = _relative_to_home(original, home_original)
        agent = str(row.get("agent") or "")
        if agent == PROJECT_AGENT:
            # Collectors before 1.12.0 recorded every project-catalog file as
            # `project`; the bundled project catalog names its agent, so old and
            # new collections report the same agents.
            home.project = True
            agent = catalog.project_agent_for(rel) or PROJECT_AGENT
        disk_path = root / archive_path
        # A collected row whose file is not under fs/ (a truncated or edited
        # archive) is a problem line, not an artifact a parser trips over.
        try:
            present = archive_path != "" and stat.S_ISREG(os.lstat(disk_path).st_mode)
            reason = "missing from the collection"
        except FileNotFoundError:
            present, reason = False, "missing from the collection"
        except OSError as error:
            present, reason = False, error_reason(error)
        if not present:
            collection.problems.append("%s: %s" % (original or archive_path, reason))
            continue
        collection.artifacts.append(
            Artifact(
                disk_path=disk_path,
                original=original,
                rel=rel,
                agent=agent,
                home=home,
                manifest=row,
            )
        )
    collection.homes = list(homes.values())


def _archive_dir_for_home(row: dict) -> str:
    """Return `fs/<home>` as the collector laid it out, derived from one manifest row.

    Deriving it from a row handles Windows drive-letter layouts
    (`fs/C/Users/alice`) without guessing.
    """
    original = str(row.get("path") or "")
    home = str(row.get("home") or "")
    archive_path = str(row.get("archive_path") or "")
    rel = _relative_to_home(original, home)
    if rel and archive_path.replace("\\", "/").endswith(rel):
        return archive_path.replace("\\", "/")[: -len(rel)].rstrip("/")
    return "fs" + home.replace("\\", "/")


def _relative_to_home(path: str, home: str) -> str:
    posix_path = path.replace("\\", "/")
    posix_home = home.replace("\\", "/").rstrip("/")
    if posix_home and (posix_path == posix_home or posix_path.startswith(posix_home + "/")):
        return posix_path[len(posix_home) :].lstrip("/")
    return posix_path.lstrip("/")


# ---- loose layout (no manifest) ------------------------------------------------


def _discover_loose(collection: Collection, catalog: Catalog) -> None:
    root = collection.root
    tree = (
        root / "fs" if (root / "fs").is_dir() and not (root / "manifest.jsonl").exists() else root
    )
    if tree is not root:
        collection.notes.append("fs/ found without manifest.jsonl; treating fs/ as the root")
    homes: list[Home] = []

    # The analyst may have pointed at an agent directory itself (~/.claude,
    # ~/.codex copied out on its own). Its parent is then the home, restricted
    # to this one directory, and nothing inside it is a home.
    for catalog_entry in catalog.entries:
        if (
            len(catalog_entry.segments) == 1
            and catalog_entry.agent != "shared"
            and catalog_entry.regexes[0].fullmatch(tree.name)
        ):
            parent = tree.resolve().parent
            home = Home(
                disk_path=tree.parent,
                original="",
                user=_user_from_parts(parent.parts[1:]),
                host=collection.host,
                inferred=True,
            )
            collection.notes.append(
                "root is a %s directory; treating its parent as the home" % catalog_entry.agent
            )
            homes.append(home)
            nested = catalog.nested_matches(tree, onerror=_problem_recorder(collection))
            _add_hits(collection, home, [(catalog_entry, tree), *list(nested)])
            collection.homes = homes
            return

    first_patterns = catalog.first_segment_regexes()
    for dirpath, dirnames, filenames in os.walk(
        tree, onerror=_problem_recorder(collection), followlinks=False
    ):
        here = Path(dirpath)
        names = dirnames + filenames
        if any(pattern.fullmatch(name) for name in names for pattern in first_patterns):
            hits = list(catalog.matches_in_home(here, onerror=_problem_recorder(collection)))
            # A directory holding only shared files (AGENTS.md) is a
            # project, not a home; it needs a hit from a real agent.
            if any(entry.agent != "shared" for entry, _ in hits):
                homes.append(_make_home(here, tree, collection.host))
                _add_hits(collection, homes[-1], hits)
                dirnames[:] = []  # a home does not contain other homes
                continue
        dirnames[:] = [dirname for dirname in dirnames if not (here / dirname).is_symlink()]
    collection.homes = homes


def _user_from_parts(parts) -> str:
    """Return the owner of a home from its path components, or `""`.

    The conventions are those the collectors enumerate homes by: home/<user>,
    Users/<user>, usr/home/<user>, export/home/<user>, root and var/root.
    """
    parts = [part for part in parts if part]
    for parent in HOME_PARENTS:
        parent_parts = parent.split("/")
        if (
            len(parts) >= len(parent_parts) + 1
            and parts[-len(parent_parts) - 1 : -1] == parent_parts
        ):
            return parts[-1]
    if "/".join(parts) in ROOT_HOMES:
        return "root"
    return ""


def _make_home(here: Path, tree: Path, host: str) -> Home:
    rel = here.relative_to(tree).as_posix() if here != tree else ""
    parts = rel.split("/") if rel else []
    user = _user_from_parts(parts)
    if not user and parts and parts[-1] != "fs":
        user = parts[-1]  # a directory that holds agent state is usually named after its owner
    return Home(
        disk_path=here, original="/" + rel if rel else "", user=user, host=host, inferred=True
    )


def _add_hits(collection: Collection, home: Home, hits) -> None:
    # The most specific entry wins: .gemini/antigravity-cli claims its files
    # before the enclosing .gemini entry sweeps the rest, matching the
    # collectors' nested-claim rule.
    hits = sorted(hits, key=lambda hit: -len(hit[0].segments))
    seen: dict[Path, None] = {}
    onerror = _problem_recorder(collection)
    for entry, hit in hits:
        for path in _walk_files(hit, onerror):
            if path in seen:
                continue
            seen[path] = None
            rel = path.relative_to(home.disk_path).as_posix()
            collection.artifacts.append(
                Artifact(
                    disk_path=path,
                    original=(home.original.rstrip("/") + "/" + rel) if home.original else rel,
                    rel=rel,
                    agent=entry.agent,
                    home=home,
                )
            )


def _problem_recorder(collection: Collection):
    """Return an os.walk `onerror` callback that records each unreadable path once as a problem."""

    def record(error: OSError) -> None:
        line = describe_error("", error)
        if line not in collection.problems:
            collection.problems.append(line)

    return record


def _walk_files(path: Path, onerror=None) -> Iterator[Path]:
    if path.is_symlink():
        return
    if path.is_file():
        yield path
        return
    for dirpath, dirnames, filenames in os.walk(path, onerror=onerror, followlinks=False):
        here = Path(dirpath)
        dirnames[:] = [dirname for dirname in dirnames if not (here / dirname).is_symlink()]
        for filename in filenames:
            file_path = here / filename
            if not file_path.is_symlink():
                yield file_path
