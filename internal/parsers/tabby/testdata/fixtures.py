"""Build the tabby package's test databases.

Run it from any directory:

    python3 -I internal/parsers/tabby/testdata/fixtures.py

Each database below is an ee/db.sqlite written by SQLite itself, so the Go tests read files
SQLite wrote. Pages are 512 bytes to keep the files small. No test runs this script; rerun it
after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

# A self-hosted integration whose api_base carries a credential.
INTEGRATIONS = """
PRAGMA page_size = 512;
CREATE TABLE integrations (id INTEGER PRIMARY KEY, kind TEXT, display_name TEXT,
    access_token TEXT, api_base TEXT, created_at TIMESTAMP);
INSERT INTO integrations VALUES (1, 'gitlab_self_hosted', 'work', 'glpat-TOKEN',
    'https://ci:s3cret@gitlab.example/api', '2026-10-01 09:00:00');
"""

# Tables of an older shape, without user_id and author_id: thread 7 and page 3 have their row,
# thread 8 and page 4 only their children.
NARROW_TABLES = """
PRAGMA page_size = 512;
CREATE TABLE threads (id INTEGER PRIMARY KEY, created_at TIMESTAMP);
INSERT INTO threads VALUES (7, '2026-10-01 09:00:00');
CREATE TABLE thread_messages (id INTEGER PRIMARY KEY, thread_id INTEGER, role TEXT, content TEXT,
    created_at TIMESTAMP);
INSERT INTO thread_messages VALUES (1, 7, 'user', 'hello', '2026-10-01 09:00:01');
INSERT INTO thread_messages VALUES (2, 8, 'user', 'orphan', '2026-10-01 09:00:02');
CREATE TABLE pages (id INTEGER PRIMARY KEY, title TEXT, created_at TIMESTAMP);
INSERT INTO pages VALUES (3, 'Notes', '2026-10-01 09:00:00');
CREATE TABLE page_sections (id INTEGER PRIMARY KEY, page_id INTEGER, title TEXT, content TEXT,
    position INTEGER, created_at TIMESTAMP);
INSERT INTO page_sections VALUES (5, 3, 'One', 'body', 0, '2026-10-01 09:00:03');
INSERT INTO page_sections VALUES (6, 4, 'Lost', 'body', 0, '2026-10-01 09:00:04');
"""

# A user whose API token is stored.
USERS = """
PRAGMA page_size = 512;
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, name TEXT, auth_token TEXT,
    created_at TIMESTAMP);
INSERT INTO users VALUES (1, 'ana@example.com', 'Ana', 'auth_FIXTURE', '2026-10-01 09:00:00');
"""

DATABASES = {"integrations.db": INTEGRATIONS, "narrow_tables.db": NARROW_TABLES, "users.db": USERS}


def repeated_ids(count: int) -> str:
    """Return the SQL of a database whose thread id 1 and page id 2 are each on count rows.

    The tables have no key, so an id can repeat; the last thread row stores its id as the text
    '1', which SQL's = finds unequal to the number 1 in a column of no declared type, so it is
    another thread. Thread 1 has count messages and page 2 count sections.
    """
    lines = [
        "PRAGMA page_size = 512;",
        "CREATE TABLE threads (id, created_at TIMESTAMP);",
        "CREATE TABLE thread_messages (id INTEGER, thread_id, role TEXT, content TEXT, created_at TIMESTAMP);",
        "CREATE TABLE pages (id, title TEXT, created_at TIMESTAMP);",
        "CREATE TABLE page_sections (id INTEGER, page_id, title TEXT, content TEXT, position INTEGER,"
        " created_at TIMESTAMP);",
    ]
    for i in range(count):
        thread = "'1'" if i == count - 1 else "1"
        lines += [
            f"INSERT INTO threads VALUES ({thread}, '2026-10-01 09:00:00');",
            f"INSERT INTO thread_messages VALUES ({i}, 1, 'user', 'message {i}', '2026-10-01 09:00:01');",
            f"INSERT INTO pages VALUES (2, 'copy {i}', '2026-10-01 09:00:00');",
            f"INSERT INTO page_sections VALUES ({i}, 2, 'section {i}', 'body', {i}, '2026-10-01 09:00:02');",
        ]
    return "\n".join(lines)


# The length of the long values below, over the 4,096 bytes a carried value is cut to.
LONG = 5000


def long_user(count: int) -> str:
    """Return the SQL of a database whose one user, of a 5,000-byte name, owns count threads."""
    lines = [
        "PRAGMA page_size = 512;",
        "CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, name TEXT, auth_token TEXT, created_at TIMESTAMP);",
        f"INSERT INTO users VALUES (1, 'ana@example.com', '{'n' * LONG}', NULL, '2026-10-01 09:00:00');",
        "CREATE TABLE threads (id INTEGER PRIMARY KEY, user_id INTEGER, created_at TIMESTAMP);",
    ]
    lines += [
        f"INSERT INTO threads VALUES ({i}, 1, '2026-10-01 09:00:00');" for i in range(1, count + 1)
    ]
    return "\n".join(lines)


def long_integration(count: int) -> str:
    """Return the SQL of a database whose one integration, of a 5,000-byte name and kind, has count repositories."""
    lines = [
        "PRAGMA page_size = 512;",
        "CREATE TABLE integrations (id INTEGER PRIMARY KEY, kind TEXT, display_name TEXT, access_token TEXT,"
        " api_base TEXT, created_at TIMESTAMP);",
        f"INSERT INTO integrations VALUES (1, '{'k' * LONG}', '{'n' * LONG}', NULL, NULL, '2026-10-01 09:00:00');",
        "CREATE TABLE provided_repositories (id INTEGER PRIMARY KEY, integration_id INTEGER, name TEXT,"
        " git_url TEXT, active INTEGER, updated_at TIMESTAMP);",
    ]
    lines += [
        f"INSERT INTO provided_repositories VALUES ({i}, 1, 'r{i}', NULL, 1, '2026-10-01 09:00:00');"
        for i in range(1, count + 1)
    ]
    return "\n".join(lines)


def scripts() -> dict[str, str]:
    """Return the SQL of every database, by file name."""
    built = dict(DATABASES)
    for count in (10, 40):
        built[f"repeated_ids_{count}.db"] = repeated_ids(count)
        built[f"long_user_{count}.db"] = long_user(count)
        built[f"long_integration_{count}.db"] = long_integration(count)
    return built


def main() -> None:
    """Rewrite every database of scripts() in this directory."""
    for name, script in scripts().items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript(script)
            connection.commit()


if __name__ == "__main__":
    main()
