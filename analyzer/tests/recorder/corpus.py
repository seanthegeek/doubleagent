"""Write the corpus: content-addressed blobs, trees and times, case lines, the root substitution.

`Corpus` owns the output directory and the substitutions; `Snapshotter` turns a directory on
disk into tree objects and a map of modification times. A tree holds no times, so the fixture
trees that every test builds afresh share their tree objects and differ only in their times.
A file that cannot be read when a snapshot is taken (mode 000) keeps its current mode and the
bytes an earlier snapshot of the same path saw, and a directory that cannot be listed keeps the
entries it last had, so a replay can recreate the denied file.
"""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import stat
from pathlib import Path
from typing import Any, TextIO

FORMAT = 1
# The temporary root is padded to this many bytes, long enough for the replay's own temporary
# directory on every runner, so the two roots can be swapped without moving any offset.
ROOT_LENGTH = 160


def placeholder(length: int) -> str:
    """Return the text of exactly `length` bytes that stands for the root inside a blob."""
    return "{parity-root".ljust(length - 1, "_") + "}"


ROOT_PLACEHOLDER = placeholder(ROOT_LENGTH)
# What stands for the root and the repository in case text, tree objects and times.
ROOT_TOKEN = "{ROOT}"
REPO_TOKEN = "{REPO}"
# The file that marks a directory as a corpus or a work directory this package may clear.
CORPUS_FILE = "corpus.json"
WORK_MARKER = ".parity-record-work"

KINDS = {
    stat.S_IFREG: "file",
    stat.S_IFDIR: "dir",
    stat.S_IFLNK: "link",
    stat.S_IFIFO: "fifo",
    stat.S_IFSOCK: "socket",
    stat.S_IFCHR: "char",
    stat.S_IFBLK: "block",
}


class RecorderError(RuntimeError):
    """The corpus or work directory cannot be used: it holds something this package did not write."""


def tagged_text(value: str) -> str | dict[str, str]:
    """Return `value` as JSON can carry it: itself, or `{"hex": ...}` when it holds a surrogate."""
    try:
        value.encode("utf-8")
    except UnicodeEncodeError:
        return {"hex": value.encode("utf-8", "surrogatepass").hex()}
    return value


def tagged_name(value: str) -> str | dict[str, str]:
    """Return a file name or link target: itself, or `{"hex": raw bytes}` when not UTF-8."""
    raw = os.fsencode(value)
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError:
        return {"hex": raw.hex()}


def _clear(path: Path, marker: str) -> None:
    """Remove `path` when it is absent, empty or holds `marker`; refuse anything else."""
    if not path.exists():
        return
    if any(path.iterdir()) and not (path / marker).exists():
        raise RecorderError("%s is not empty and holds no %s" % (path, marker))
    # A test may have left a directory at mode 000: os.walk lists a directory before it descends,
    # so opening each child directory up as its parent is listed reaches every one.
    os.chmod(path, 0o700)
    for directory, names, _ in os.walk(path):
        for name in names:
            child = os.path.join(directory, name)
            if not os.path.islink(child):
                os.chmod(child, 0o700)
    shutil.rmtree(path)


def make_work_root(work: Path, length: int = ROOT_LENGTH) -> str:
    """Create the work directory and the root inside it, padded to `length` bytes; return it.

    Raises:
        RecorderError: The work directory holds files of something else, or its path is too
            long to pad to `ROOT_LENGTH`.
    """
    _clear(work, WORK_MARKER)
    work.mkdir(parents=True)
    (work / WORK_MARKER).write_text("parity recording work directory\n", encoding="utf-8")
    base = os.path.realpath(work)
    padding = length - len(os.fsencode(base)) - 1
    if padding < 1:
        raise RecorderError("%s is too long to pad to %d bytes" % (base, length))
    root = os.path.join(base, "r" * padding)
    os.mkdir(root)
    return root


def remove_work(work: Path) -> None:
    """Remove the work directory, whatever modes the tests left inside it."""
    _clear(work, WORK_MARKER)


class Corpus:
    """The output directory: blobs, trees, times, case files and the substitutions of text."""

    def __init__(self, directory: Path, root: str, repo: str) -> None:
        """Clear `directory` (absent, empty or an earlier corpus) and start writing into it."""
        _clear(directory, CORPUS_FILE)
        self.directory = directory
        self.root = root
        self.repo = repo
        self.root_bytes = os.fsencode(root)
        self.placeholder_bytes = placeholder(len(self.root_bytes)).encode("ascii")
        for name in ("blobs", "trees", "times", "parsers", "commands"):
            (directory / name).mkdir(parents=True)
        self.blobs: set[str] = set()
        self.trees: dict[str, list[dict[str, Any]]] = {}
        self.known_trees: dict[str, str] = {}
        self.times_written: set[str] = set()
        self.files: dict[str, TextIO] = {}
        self.counts: dict[str, int] = {}
        header = {
            "format": FORMAT,
            "root_length": len(self.root_bytes),
            "root_placeholder": self.placeholder_bytes.decode("ascii"),
            "root_token": ROOT_TOKEN,
            "repo_token": REPO_TOKEN,
        }
        (directory / CORPUS_FILE).write_text(dumps(header) + "\n", encoding="utf-8")

    def text(self, value: str) -> str:
        """Return text with the root and the repository replaced by their tokens."""
        value = value.replace(self.root, ROOT_TOKEN)
        return value.replace(self.repo, REPO_TOKEN) if self.repo else value

    def portable(self, value: Any) -> Any:
        """Return a JSON-ready copy of `value`, every text substituted and tagged."""
        if isinstance(value, str):
            return tagged_text(self.text(value))
        if isinstance(value, os.PathLike):
            return tagged_text(self.text(os.fspath(value)))
        if isinstance(value, dict):
            return {_key(self.text(str(key))): self.portable(item) for key, item in value.items()}
        if isinstance(value, list | tuple):
            return [self.portable(item) for item in value]
        if value is None or isinstance(value, bool | int | float):
            return value
        return tagged_text(self.text(repr(value)))

    def blob(self, data: bytes) -> str:
        """Store bytes with the root replaced by its placeholder, once; return their hash."""
        data = data.replace(self.root_bytes, self.placeholder_bytes)
        digest = hashlib.sha256(data).hexdigest()
        if digest not in self.blobs:
            (self.directory / "blobs" / digest).write_bytes(data)
            self.blobs.add(digest)
        return digest

    def tree(self, entries: list[dict[str, Any]]) -> str:
        """Store one directory's entries, once; return the hash of the tree file."""
        text = dumps({"entries": entries}) + "\n"
        known = self.known_trees.get(text)
        if known:
            return known
        digest = hashlib.sha256(text.encode("utf-8")).hexdigest()
        self.known_trees[text] = digest
        if digest not in self.trees:
            (self.directory / "trees" / (digest + ".json")).write_text(text, encoding="utf-8")
            self.trees[digest] = entries
        return digest

    def times(self, mtimes: dict[str, int]) -> str:
        """Store a snapshot's modification times by root-relative path, once; return the hash."""
        text = dumps({"mtime_ns": dict(sorted(mtimes.items()))}) + "\n"
        digest = hashlib.sha256(text.encode("utf-8")).hexdigest()
        if digest not in self.times_written:
            (self.directory / "times" / (digest + ".json")).write_text(text, encoding="utf-8")
            self.times_written.add(digest)
        return digest

    def write_case(self, group: str, case: dict[str, Any]) -> None:
        """Append one case to the file of `group` (`parsers/<agent>`, `commands/<name>`, ...)."""
        handle = self.files.get(group)
        if handle is None:
            # Kept open for the whole run and closed by close(), so no context manager.
            path = self.directory / (group + ".jsonl")
            handle = open(path, "w", encoding="utf-8", newline="")  # noqa: SIM115
            self.files[group] = handle
        handle.write(dumps(case) + "\n")
        self.counts[group] = self.counts.get(group, 0) + 1

    def close(self, silent_tests: list[str]) -> None:
        """Close the case files and write `summary.json`."""
        for handle in self.files.values():
            handle.close()
        summary = {
            "blobs": len(self.blobs),
            "trees": len(self.trees),
            "times": len(self.times_written),
            "cases": dict(sorted(self.counts.items())),
            "tests_without_cases": sorted(silent_tests),
        }
        (self.directory / "summary.json").write_text(dumps(summary, 1) + "\n", encoding="utf-8")


def _key(text: str) -> str:
    """Return a JSON object key; a key holding a surrogate keeps it as a replacement character."""
    return text.encode("utf-8", "surrogatepass").decode("utf-8", "replace")


def dumps(value: Any, indent: int | None = None) -> str:
    """Return the corpus's one JSON form: sorted keys, UTF-8 kept, no spaces unless indented."""
    separators = (",", ": ") if indent else (",", ":")
    return json.dumps(
        value, sort_keys=True, ensure_ascii=False, separators=separators, indent=indent
    )


class Snapshot:
    """One root entry as a snapshot saw it: its tree entry, the times below it, what it walked."""

    def __init__(
        self,
        entry: dict[str, Any] | None,
        mtimes: dict[str, int],
        walked: list[tuple[str, tuple[int, ...] | str]],
    ) -> None:
        """Hold the entry (None when the path did not exist), the times and each path's status."""
        self.entry = entry
        self.mtimes = mtimes
        self.walked = walked

    def current(self) -> bool:
        """Return whether every path the snapshot walked still has the status it saw."""
        return all(current_key(path) == key for path, key in self.walked)


class Snapshotter:
    """Turn directories into trees and times, reusing the hash of a file whose status is unchanged."""

    def __init__(self, corpus: Corpus) -> None:
        """Start with no file or directory seen."""
        self.corpus = corpus
        # path -> (status key, blob hash) of the last read of that file
        self.files: dict[str, tuple[tuple[int, ...], str]] = {}
        # path -> (tree hash, times below it) of the last listing of that directory
        self.directories: dict[str, tuple[str, dict[str, int]]] = {}
        # path -> status key, for every entry of the latest snapshot
        self.seen: dict[str, tuple[int, ...]] = {}
        # path -> (tree entry, times from it down) of the latest snapshot of that path
        self.entries: dict[str, tuple[dict[str, Any] | None, dict[str, int]]] = {}
        # (path, status key or "missing" or "denied") of each path the current walk met
        self.walked: list[tuple[str, tuple[int, ...] | str]] = []

    def root_entry(self, name: str) -> Snapshot:
        """Return the snapshot of one entry of the root, its times keyed from the root."""
        mtimes: dict[str, int] = {}
        self.walked = []
        try:
            entry = self.entry(os.path.join(self.corpus.root, name), name, name, mtimes)
            return Snapshot(entry, mtimes, self.walked)
        finally:
            self.walked = []

    def entry(
        self, path: str, name: str, relative: str, mtimes: dict[str, int]
    ) -> dict[str, Any] | None:
        """Return the tree entry of `path`, adding its time and those below it to `mtimes`.

        An entry in a directory that can be listed but not searched keeps its last snapshot.
        """
        try:
            status = os.lstat(path)
        except FileNotFoundError:
            self.walked.append((path, "missing"))
            return None
        except PermissionError:
            self.walked.append((path, "denied"))
            entry, below = self.entries.get(path, (None, {}))
            mtimes.update(below)
            return entry
        key = _status_key(status)
        self.seen[path] = key
        self.walked.append((path, key))
        kind = KINDS.get(stat.S_IFMT(status.st_mode), "other")
        entry: dict[str, Any] = {
            "name": tagged_name(name),
            "kind": kind,
            "mode": stat.S_IMODE(status.st_mode),
        }
        below = {_relative_key(relative): status.st_mtime_ns}
        if kind == "file":
            entry["blob"] = self.file(path, key)
        elif kind == "dir":
            entry["tree"] = self.directory(path, relative, below)
        elif kind == "link":
            entry["target"] = tagged_name(self.corpus.text(os.readlink(path)))
        self.entries[path] = (entry, below)
        mtimes.update(below)
        return entry

    def file(self, path: str, key: tuple[int, ...]) -> str | None:
        """Return the blob hash of a file, or the last one seen when it cannot be read now."""
        known = self.files.get(path)
        if known and known[0] == key:
            return known[1]
        try:
            with open(path, "rb") as handle:
                data = handle.read()
        except PermissionError:
            return known[1] if known else None
        digest = self.corpus.blob(data)
        self.files[path] = (key, digest)
        return digest

    def directory(self, path: str, relative: str, mtimes: dict[str, int]) -> str:
        """Return the tree hash of a directory, or its last one when it cannot be listed now."""
        try:
            names = os.listdir(path)
        except PermissionError:
            tree, below = self.directories.get(path, (self.corpus.tree([]), {}))
            mtimes.update(below)
            return tree
        below: dict[str, int] = {}
        entries = []
        for name in sorted(names, key=os.fsencode):
            entry = self.entry(os.path.join(path, name), name, relative + "/" + name, below)
            if entry is not None:
                entries.append(entry)
        digest = self.corpus.tree(entries)
        self.directories[path] = (digest, below)
        mtimes.update(below)
        return digest

    def unchanged(self, path: str) -> bool:
        """Return whether `path` has the status the latest snapshot saw."""
        try:
            return self.seen.get(path) == _status_key(os.lstat(path))
        except OSError:
            return False


def _relative_key(relative: str) -> str:
    """Return a root-relative path as a times key: itself, or its bytes in hex when not UTF-8."""
    tagged = tagged_name(relative)
    return tagged if isinstance(tagged, str) else "hex:" + tagged["hex"]


def current_key(path: str) -> tuple[int, ...] | str:
    """Return the status key of `path` now, or "missing" or "denied" when it cannot be had."""
    try:
        return _status_key(os.lstat(path))
    except FileNotFoundError:
        return "missing"
    except PermissionError:
        return "denied"


def _status_key(status: os.stat_result) -> tuple[int, ...]:
    """Return what tells a changed file apart: device, inode, mode, size and modification time.

    The change time is left out: a test that locks a file and unlocks it again moves it, and a
    file read again would be an `open` the suite's audit hook counts as the parser's.
    """
    return (status.st_dev, status.st_ino, status.st_mode, status.st_size, status.st_mtime_ns)


def top_of(root: str, path: str) -> str:
    """Return the first component of `path` under `root`, or "" when it lies elsewhere."""
    prefix = root + os.sep
    if not path.startswith(prefix):
        return ""
    return path[len(prefix) :].split(os.sep, 1)[0]


def tree_files(trees: dict[str, list[dict[str, Any]]], digest: str, prefix: str = "") -> dict:
    """Return every file and link under a tree as path -> (kind, blob or target)."""
    found: dict[str, tuple[str, Any]] = {}
    for entry in trees.get(digest, []):
        name = entry["name"] if isinstance(entry["name"], str) else "hex:" + entry["name"]["hex"]
        path = prefix + name
        if entry["kind"] == "dir":
            found.update(tree_files(trees, entry["tree"], path + "/"))
        elif entry["kind"] == "file":
            found[path] = ("file", entry["blob"])
        elif entry["kind"] == "link":
            found[path] = ("link", entry["target"])
    return found
