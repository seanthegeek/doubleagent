"""Tests for extract.py: a pulled archive gives its files and directories, never a link."""

from __future__ import annotations

import io
import pathlib
import tarfile
import zipfile

import extract


def add_tar_member(archive: tarfile.TarFile, member: tarfile.TarInfo, data: bytes = b"") -> None:
    """Add one member, with its bytes when it is a regular file."""
    member.size = len(data)
    archive.addfile(member, io.BytesIO(data) if data else None)


def write_tar(path: pathlib.Path) -> None:
    """Write a tar.gz with a file, a directory, a symlink and a hard link."""
    with tarfile.open(path, "w:gz") as archive:
        directory = tarfile.TarInfo("fs/home/alice")
        directory.type = tarfile.DIRTYPE
        add_tar_member(archive, directory)
        add_tar_member(archive, tarfile.TarInfo("manifest.jsonl"), b'{"status":"collected"}\n')
        link = tarfile.TarInfo("fs/home/alice/link")
        link.type, link.linkname = tarfile.SYMTYPE, "/etc/passwd"
        add_tar_member(archive, link)
        hard = tarfile.TarInfo("fs/home/alice/hard")
        hard.type, hard.linkname = tarfile.LNKTYPE, "manifest.jsonl"
        add_tar_member(archive, hard)


def test_a_tar_gives_its_files_and_drops_every_link(tmp_path: pathlib.Path):
    archive = tmp_path / "collection.tar.gz"
    write_tar(archive)
    destination = tmp_path / "extract"
    assert extract.main([str(archive), str(destination)]) == 0
    assert (destination / "manifest.jsonl").read_bytes() == b'{"status":"collected"}\n'
    assert (destination / "fs/home/alice").is_dir()
    assert not (destination / "fs/home/alice/link").exists()
    assert not (destination / "fs/home/alice/link").is_symlink()
    assert not (destination / "fs/home/alice/hard").exists()


def test_a_tar_member_outside_the_destination_is_refused(tmp_path: pathlib.Path):
    archive = tmp_path / "escape.tar"
    with tarfile.open(archive, "w") as handle:
        add_tar_member(handle, tarfile.TarInfo("../outside"), b"x")
    assert extract.main([str(archive), str(tmp_path / "extract")]) == 1
    assert not (tmp_path / "outside").exists()


def test_a_zip_drops_absolute_parent_and_drive_names(tmp_path: pathlib.Path):
    archive = tmp_path / "collection.zip"
    with zipfile.ZipFile(archive, "w") as handle:
        handle.writestr("manifest.jsonl", "{}\n")
        handle.writestr("fs/Users/alice/", "")
        handle.writestr("fs\\Users\\alice\\note.txt", "note")
        handle.writestr("../outside.txt", "x")
        handle.writestr("/absolute.txt", "x")
        handle.writestr("C:/drive.txt", "x")
    destination = tmp_path / "extract"
    assert extract.main([str(archive), str(destination)]) == 0
    assert (destination / "fs/Users/alice/note.txt").read_text() == "note"
    names = sorted(str(path.relative_to(destination)) for path in destination.rglob("*"))
    assert names == [
        "fs",
        "fs/Users",
        "fs/Users/alice",
        "fs/Users/alice/note.txt",
        "manifest.jsonl",
    ]
    assert not (tmp_path / "outside.txt").exists()


def test_a_file_that_is_no_archive_fails_with_its_reason(tmp_path: pathlib.Path, capsys):
    plain = tmp_path / "plain.txt"
    plain.write_text("not an archive")
    assert extract.main([str(plain), str(tmp_path / "extract")]) == 1
    assert "neither a tar nor a zip archive" in capsys.readouterr().err
