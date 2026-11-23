"""Write the editor state database the pearai package's tests read.

Run from this directory with any Python 3: `python3 -I gen_state.py`.

- state.vscdb: an ItemTable whose pearai.pearai-roo-cline row holds a
  taskHistory of 200 tasks, t0 to t199, then a second entry for t7 and an
  entry that is not an object.
"""

import json
import os
import sqlite3

NAME = "state.vscdb"
HISTORY = [{"id": "t%d" % i, "task": "task %d" % i, "ts": 1791021600000 + i} for i in range(200)]
HISTORY += [{"id": "t7", "task": "a later copy"}, 5]


def main() -> None:
    """Write state.vscdb afresh."""
    if os.path.exists(NAME):
        os.remove(NAME)
    connection = sqlite3.connect(NAME)
    connection.execute("CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)")
    connection.execute(
        "INSERT INTO ItemTable VALUES (?, ?)",
        ("pearai.pearai-roo-cline", json.dumps({"taskHistory": HISTORY})),
    )
    connection.commit()
    connection.execute("VACUUM")
    connection.close()


if __name__ == "__main__":
    main()
