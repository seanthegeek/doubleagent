"""Tests of `sqlite_util.sqlite_problem`, the shared unreadable message and `read_items`."""

import sqlite3
import unittest
from pathlib import Path

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import vscode_state
from doubleagent.errors import unreadable_message
from doubleagent.sqlite_util import NOT_SQLITE, open_copy, sqlite_problem, table_names

SIDECARS = ("-wal", "-shm", "-journal")
DENIED = "could not be read (Permission denied)"


def _database(path: Path) -> Path:
    connection = sqlite3.connect(str(path))
    try:
        connection.execute("create table ItemTable (key text, value blob)")
        connection.execute("insert into ItemTable values ('k', '{\"a\": 1}')")
        connection.commit()
    finally:
        connection.close()
    return path


def _require_deny():
    if not CAN_DENY:
        raise unittest.SkipTest(DENY_REASON)


def test_readable_database_without_sidecars_has_no_problem(tmp_path):
    path = _database(tmp_path / "sessions.db")
    assert not any(Path(str(path) + suffix).exists() for suffix in SIDECARS)
    assert sqlite_problem(path) == ""


def test_readable_sidecars_are_no_problem(tmp_path):
    path = _database(tmp_path / "sessions.db")
    for suffix in SIDECARS:
        Path(str(path) + suffix).write_bytes(b"")
    assert sqlite_problem(path) == ""


def test_file_without_the_header_is_not_sqlite(tmp_path):
    path = tmp_path / "text.db"
    path.write_bytes(b"not a database\n")
    assert sqlite_problem(path) == NOT_SQLITE == "not a SQLite database"


def test_empty_file_is_an_empty_database(tmp_path):
    path = tmp_path / "empty.db"
    path.write_bytes(b"")
    assert sqlite_problem(path) == ""
    with open_copy(path) as connection:
        assert table_names(connection) == set()


def test_unreadable_database_names_the_file_and_reason(tmp_path):
    _require_deny()
    path = _database(tmp_path / "sessions.db")
    path.chmod(0)
    try:
        assert sqlite_problem(path) == "sessions.db " + DENIED
    finally:
        path.chmod(0o600)


def test_each_unreadable_sidecar_names_the_sidecar(tmp_path):
    _require_deny()
    path = _database(tmp_path / "sessions.db")
    for suffix in SIDECARS:
        Path(str(path) + suffix).write_bytes(b"")
    for suffix in SIDECARS:
        sidecar = Path(str(path) + suffix)
        sidecar.chmod(0)
        try:
            assert sqlite_problem(path) == "sessions.db%s %s" % (suffix, DENIED), suffix
        finally:
            sidecar.chmod(0o600)
        assert sqlite_problem(path) == "", suffix


def test_missing_database_is_a_problem(tmp_path):
    assert sqlite_problem(tmp_path / "absent.db") == (
        "absent.db could not be read (No such file or directory)"
    )


def test_unreadable_message_falls_back_to_the_error_text():
    cases = (
        (PermissionError(13, "Permission denied"), "a.db could not be read (Permission denied)"),
        (OSError("disk gone"), "a.db could not be read (disk gone)"),
    )
    for error, want in cases:
        assert unreadable_message(Path("/x/a.db"), error) == want, error


def test_read_items_reports_a_file_it_cannot_open(tmp_path):
    path = tmp_path / "state.vscdb"
    assert vscode_state.read_items(path, ["k"]) == ({}, []), "absent"
    path.write_bytes(b"plain text\n")
    assert vscode_state.read_items(path, ["k"]) == ({}, ["state.vscdb is not a SQLite database"])
    path.unlink()
    _database(path)
    assert vscode_state.read_items(path, ["k"]) == ({"k": {"a": 1}}, [])


def test_read_items_reports_an_unreadable_database_or_sidecar(tmp_path):
    _require_deny()
    path = _database(tmp_path / "state.vscdb")
    wal = Path(str(path) + "-wal")
    wal.write_bytes(b"")
    for locked in (path, wal):
        locked.chmod(0)
        try:
            values, problems = vscode_state.read_items(path, ["k"])
        finally:
            locked.chmod(0o600)
        assert (values, problems) == ({}, ["%s %s" % (locked.name, DENIED)]), locked.name


def test_shown_problem_cuts_only_a_value_decoder_detail():
    cases = (
        (
            "ItemTable value for k is not valid JSON (Expecting value: line 1 column 1 (char 0))",
            "ItemTable value for k is not valid JSON",
        ),
        ("state.vscdb " + DENIED, "state.vscdb " + DENIED),
        ("state.vscdb-wal " + DENIED, "state.vscdb-wal " + DENIED),
    )
    for problem, want in cases:
        assert vscode_state.shown_problem(problem) == want, problem
