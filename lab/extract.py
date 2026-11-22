#!/usr/bin/env python3
"""Extract a pulled collection archive without creating a link or writing outside DEST.

    python3 -I lab/extract.py ARCHIVE DEST

The archive is built in the VM from directories the agent under test controls, so it is never
unpacked with tar or unzip: only regular files and directories are written, no link or device
member is created, and a member with an absolute path, a `..` component or a drive letter is
refused or dropped. The collector's symlinks are recorded in the manifest's `target` field, so
nothing the report reads is lost. Exits 1 with the reason on stderr when the archive cannot be
extracted. Standard library only.
"""

from __future__ import annotations

import re
import shutil
import sys
import tarfile
import zipfile
from pathlib import Path, PurePosixPath


class ExtractError(RuntimeError):
    """The file is neither a tar nor a zip archive, or cannot be extracted."""


def extract_archive(path: Path, destination: Path) -> None:
    """Extract a tar or zip archive into `destination`, its files and directories only.

    Raises:
        OSError: The archive cannot be read or a member cannot be written.
        ExtractError: The file is neither a tar nor a zip archive, or cannot be extracted.
    """
    try:
        if zipfile.is_zipfile(path):
            _extract_zip(path, destination)
        elif tarfile.is_tarfile(path):
            _extract_tar(path, destination)
        else:
            raise ExtractError("%s is neither a tar nor a zip archive" % path)
    except (tarfile.TarError, zipfile.BadZipFile, EOFError) as error:
        raise ExtractError("%s: cannot extract: %s" % (path, error)) from None


def safe_member(name: str) -> bool:
    """Return whether a member name stays below the destination: relative, no `..`, no drive."""
    member_path = PurePosixPath(name)
    return (
        not member_path.is_absolute()
        and ".." not in member_path.parts
        and not re.match(r"^[A-Za-z]:", name)
    )


def _skip_links(member: tarfile.TarInfo, destination: str) -> tarfile.TarInfo | None:
    """Drop a link or device member and pass the rest through the tar module's data filter."""
    if member.issym() or member.islnk() or member.isdev():
        return None
    return tarfile.data_filter(member, destination)


def _extract_tar(path: Path, destination: Path) -> None:
    """Extract the regular files and directories of a tar archive."""
    with tarfile.open(path, "r:*") as archive:
        if hasattr(tarfile, "data_filter"):
            archive.extractall(destination, filter=_skip_links)
            return
        members = [
            member
            for member in archive.getmembers()
            if safe_member(member.name) and not (member.issym() or member.islnk() or member.isdev())
        ]
        archive.extractall(destination, members=members)


def _extract_zip(path: Path, destination: Path) -> None:
    """Extract the files and directories of a zip archive, dropping an unsafe name."""
    with zipfile.ZipFile(path) as archive:
        for info in archive.infolist():
            name = info.filename.replace("\\", "/")
            if not safe_member(name):
                continue
            target = destination / name
            if name.endswith("/"):
                target.mkdir(parents=True, exist_ok=True)
                continue
            target.parent.mkdir(parents=True, exist_ok=True)
            with archive.open(info) as source, open(target, "wb") as written:
                shutil.copyfileobj(source, written)


def main(arguments: list[str]) -> int:
    """Run the command line; return the exit status."""
    if len(arguments) != 2:
        print("usage: extract.py ARCHIVE DEST", file=sys.stderr)
        return 2
    destination = Path(arguments[1])
    destination.mkdir(parents=True, exist_ok=True)
    try:
        extract_archive(Path(arguments[0]), destination)
    except (ExtractError, OSError) as error:
        print("extract.py: %s" % error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
