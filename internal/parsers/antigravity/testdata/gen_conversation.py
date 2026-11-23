"""Write the conversation database the antigravity package's tests read.

Run from this directory with any Python 3: `python3 -I gen_conversation.py`.

- undecodable.db: a conversation whose one step decodes, whose trajectory
  metadata blob does not, and whose gen_metadata holds two rows that do
  not decode and one empty row.
"""

import os
import sqlite3

NAME = "undecodable.db"
# A field tag that claims more varint bytes than the blob holds.
BROKEN = b"\x0a\xff\xff\xff"


def main() -> None:
    """Write undecodable.db afresh."""
    if os.path.exists(NAME):
        os.remove(NAME)
    connection = sqlite3.connect(NAME)
    connection.execute(
        "CREATE TABLE steps (idx INTEGER, step_type INTEGER, status INTEGER, metadata BLOB,"
        " step_payload BLOB)"
    )
    connection.execute("INSERT INTO steps VALUES (1, 0, 3, NULL, NULL)")
    connection.execute("CREATE TABLE trajectory_metadata_blob (data BLOB)")
    connection.execute("INSERT INTO trajectory_metadata_blob VALUES (?)", (BROKEN,))
    connection.execute("CREATE TABLE gen_metadata (data BLOB)")
    for data in (BROKEN, BROKEN, b""):
        connection.execute("INSERT INTO gen_metadata VALUES (?)", (data,))
    connection.commit()
    connection.execute("VACUUM")
    connection.close()


if __name__ == "__main__":
    main()
