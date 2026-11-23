"""Write the editor state databases the roocode package's tests read.

Run from this directory with any Python 3: `python3 -I gen_state.py`.
Each holds an ItemTable whose RooVeterinaryInc.roo-cline row is an object
whose roo.retryQueue is:

- retry-queue-number.vscdb: a number.
- retry-queue-entries.vscdb: a list holding one entry that is not an object.
"""

import json
import os
import sqlite3

DATABASES = {
    "retry-queue-number.vscdb": {"roo.retryQueue": 5},
    "retry-queue-entries.vscdb": {"roo.retryQueue": [7]},
}


def main() -> None:
    """Write each database afresh."""
    for name, value in DATABASES.items():
        if os.path.exists(name):
            os.remove(name)
        connection = sqlite3.connect(name)
        connection.execute(
            "CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)"
        )
        connection.execute(
            "INSERT INTO ItemTable VALUES (?, ?)",
            ("RooVeterinaryInc.roo-cline", json.dumps(value)),
        )
        connection.commit()
        connection.execute("VACUUM")
        connection.close()


if __name__ == "__main__":
    main()
