"""Build the opencode package's test databases.

Run it from any directory:

    python3 -I internal/parser/opencode/testdata/fixtures.py

Each database below is rewritten from its SQL by SQLite itself, so the Go tests read files
SQLite wrote. Pages are 512 bytes to keep the files small. No test runs this script; rerun it
after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

# A 1.x database: one session with one user message.
ONE_SESSION = """
CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, time_created integer,
    time_updated integer);
INSERT INTO project VALUES ('p1', '/work/proj', 1700000000000, 1700000000000);
CREATE TABLE session (id text PRIMARY KEY, project_id text, parent_id text, slug text,
    directory text, title text, version text, time_created integer, time_updated integer);
INSERT INTO session VALUES ('ses_1', 'p1', NULL, 'slug', '/work/proj', 'Fix bug', '1.0.0',
    1700000000000, 1700000001000);
CREATE TABLE message (id text PRIMARY KEY, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO message VALUES ('msg_1', 'ses_1', 1700000000500, 1700000000500,
    '{"role":"user","time":{"created":1700000000500}}');
CREATE TABLE part (id text PRIMARY KEY, message_id text, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO part VALUES ('prt_1', 'msg_1', 'ses_1', 1700000000500, 1700000000500,
    '{"type":"text","text":"hello there"}');
"""

# A 1.x database whose session is shared: the share's URL carries a credential beside its secret,
# and an event records the share of another session.
SHARED_SESSION = (
    ONE_SESSION
    + """
CREATE TABLE session_share (session_id text PRIMARY KEY, id text, secret text, url text,
    time_created integer, time_updated integer);
INSERT INTO session_share VALUES ('ses_1', 'shr_1', 'FIXTURE-SHARE-SECRET',
    'https://opncd.example/s/shr_1?token=s3cret', 1700000002000, 1700000002000);
CREATE TABLE event (id text PRIMARY KEY, aggregate_id text, seq integer, type text, data text);
INSERT INTO event VALUES ('evt_1', 'ses_2', 1, 'session.updated.1',
    '{"info":{"share":{"url":"https://ci:s3cret@opncd.example/s/shr_2"},"time":{"updated":1700000003000}}}');
"""
)



def repeated_v2_message(count: int) -> str:
    """Return a 2.x database whose session_message holds count rows of one id, as only damage
    writes, and that id's count session_part items."""
    rows = [
        """CREATE TABLE session_v2 (id text PRIMARY KEY, project_id text, directory text,
    title text, time_created integer, time_updated integer);
INSERT INTO session_v2 VALUES ('ses_1', NULL, '/work/proj', 'Repeat', 1700000000000,
    1700000000000);
CREATE TABLE session_message (id text, session_id text, type text, seq integer,
    time_created integer, time_updated integer, data text);
CREATE TABLE session_part (id text, message_id text, session_id text, type text,
    ordinal integer, data text);"""
    ]
    for i in range(count):
        rows.append(
            "INSERT INTO session_message VALUES ('msg_1', 'ses_1', 'assistant', %d, %d, %d, "
            "'{\"content\":[]}');" % (i, 1700000001000 + i, 1700000001000 + i)
        )
        rows.append(
            "INSERT INTO session_part VALUES ('prt_%d', 'msg_1', 'ses_1', 'text', %d, "
            "'{\"text\":\"part %d\"}');" % (i, i, i)
        )
    return "\n".join(rows)


def repeated_v1_message(count: int) -> str:
    """Return a 1.x database whose message table holds count rows of one id, as only damage
    writes, and that id's count parts."""
    rows = [
        """CREATE TABLE session (id text PRIMARY KEY, project_id text, directory text,
    title text, time_created integer, time_updated integer);
INSERT INTO session VALUES ('ses_1', NULL, '/work/proj', 'Repeat', 1700000000000,
    1700000000000);
CREATE TABLE message (id text, session_id text, time_created integer, time_updated integer,
    data text);
CREATE TABLE part (id text, message_id text, session_id text, time_created integer,
    time_updated integer, data text);"""
    ]
    for i in range(count):
        rows.append(
            "INSERT INTO message VALUES ('msg_1', 'ses_1', %d, %d, '{\"role\":\"assistant\"}');"
            % (1700000001000 + i, 1700000001000 + i)
        )
        rows.append(
            "INSERT INTO part VALUES ('prt_%03d', 'msg_1', 'ses_1', %d, %d, "
            "'{\"type\":\"text\",\"text\":\"part %d\"}');"
            % (i, 1700000001000 + i, 1700000001000 + i, i)
        )
    return "\n".join(rows)


# A 1.x message table whose id column compares without case, so three spellings of one id join
# the same parts.
CASELESS_MESSAGE_ID = """
CREATE TABLE session (id text PRIMARY KEY, project_id text, directory text, title text,
    time_created integer, time_updated integer);
INSERT INTO session VALUES ('ses_1', NULL, '/work/proj', 'Repeat', 1700000000000,
    1700000000000);
CREATE TABLE message (id text COLLATE NOCASE, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO message VALUES ('msg_1', 'ses_1', 1700000001000, 1700000001000,
    '{"role":"assistant"}');
INSERT INTO message VALUES ('MSG_1', 'ses_1', 1700000002000, 1700000002000,
    '{"role":"assistant"}');
INSERT INTO message VALUES ('Msg_1', 'ses_1', 1700000003000, 1700000003000,
    '{"role":"assistant"}');
CREATE TABLE part (id text, message_id text, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO part VALUES ('prt_0', 'msg_1', 'ses_1', 1700000001000, 1700000001000,
    '{"type":"text","text":"part 0"}');
INSERT INTO part VALUES ('prt_1', 'msg_1', 'ses_1', 1700000001000, 1700000001000,
    '{"type":"text","text":"part 1"}');
"""

# A 1.x session with no directory and no project whose messages ran in different directories,
# and one that names none.
MESSAGE_DIRECTORIES = """
CREATE TABLE session (id text PRIMARY KEY, project_id text, directory text, title text,
    time_created integer, time_updated integer);
INSERT INTO session VALUES ('ses_1', NULL, NULL, 't', 1700000000000, 1700000000000);
CREATE TABLE message (id text PRIMARY KEY, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO message VALUES ('msg_1', 'ses_1', 1700000001000, 1700000001000,
    '{"role":"assistant","path":{"cwd":"/a"}}');
INSERT INTO message VALUES ('msg_2', 'ses_1', 1700000002000, 1700000002000,
    '{"role":"assistant","path":{"cwd":"/b"}}');
INSERT INTO message VALUES ('msg_3', 'ses_1', 1700000003000, 1700000003000,
    '{"role":"assistant"}');
CREATE TABLE part (id text PRIMARY KEY, message_id text, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO part VALUES ('prt_1', 'msg_1', 'ses_1', 1700000001000, 1700000001000,
    '{"type":"text","text":"from msg_1"}');
INSERT INTO part VALUES ('prt_2', 'msg_2', 'ses_1', 1700000002000, 1700000002000,
    '{"type":"text","text":"from msg_2"}');
INSERT INTO part VALUES ('prt_3', 'msg_3', 'ses_1', 1700000003000, 1700000003000,
    '{"type":"text","text":"from msg_3"}');
"""

# Sessions whose permission columns hold entries that are not rules beside rules.
PERMISSION_ENTRIES = """
CREATE TABLE session_v2 (id text PRIMARY KEY, project_id text, directory text, title text,
    permission text, time_created integer, time_updated integer);
INSERT INTO session_v2 VALUES ('ses_2', NULL, '/work/two', 't2',
    '[1, "x", {"action":"read","resource":"*","effect":"allow"}, {"other":1}]',
    1700000000000, 1700000005000);
CREATE TABLE session (id text PRIMARY KEY, project_id text, directory text, title text,
    permission text, time_created integer, time_updated integer);
INSERT INTO session VALUES ('ses_1', NULL, '/work/one', 't1',
    '[{"permission":"bash","pattern":"ls","action":"ask"}, null]',
    1700000001000, 1700000006000);
"""

DATABASES = {
    "no_permission_table.db": ONE_SESSION,
    # The first layout of the permission table, which every early 1.x database had.
    "first_layout_permission.db": ONE_SESSION
    + "CREATE TABLE permission (project_id text PRIMARY KEY, data text NOT NULL);",
    "first_layout_permission_with_times.db": ONE_SESSION
    + """CREATE TABLE permission (project_id text PRIMARY KEY, data text NOT NULL,
        time_created integer, time_updated integer);
    INSERT INTO permission VALUES ('p1', '{}', 1700000000000, 1700000000000);""",
    "shared_session.db": SHARED_SESSION,
    "repeated_message_id_10.db": repeated_v2_message(10),
    "repeated_message_id_40.db": repeated_v2_message(40),
    "repeated_v1_message_id_10.db": repeated_v1_message(10),
    "repeated_v1_message_id_40.db": repeated_v1_message(40),
    "caseless_message_id.db": CASELESS_MESSAGE_ID,
    "message_directories.db": MESSAGE_DIRECTORIES,
    "permission_entries.db": PERMISSION_ENTRIES,
}


def main() -> None:
    """Rewrite every database of DATABASES in this directory."""
    for name, script in DATABASES.items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript("PRAGMA page_size = 512;" + script)
            connection.commit()


if __name__ == "__main__":
    main()
