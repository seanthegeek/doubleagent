"""Write the editor state databases the tabby package's tests read.

Run from this directory with any Python 3: `python3 -I gen_state.py`.
Each holds an ItemTable whose TabbyML.vscode-tabby row is an object whose
edit.recentlyCommand member is, by file name:

- command-string.vscdb: a string.
- command-number.vscdb: a number.
- command-object.vscdb: an object.
- command-null.vscdb: null.
- command-empty.vscdb: an empty array.
"""

import json
import os
import sqlite3

DATABASES = {
    "command-string.vscdb": {"edit.recentlyCommand": "x"},
    "command-number.vscdb": {"edit.recentlyCommand": 7},
    "command-object.vscdb": {"edit.recentlyCommand": {"command": "go"}},
    "command-null.vscdb": {"edit.recentlyCommand": None},
    "command-empty.vscdb": {"edit.recentlyCommand": []},
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
            "INSERT INTO ItemTable VALUES (?, ?)", ("TabbyML.vscode-tabby", json.dumps(value))
        )
        connection.commit()
        connection.execute("VACUUM")
        connection.close()


if __name__ == "__main__":
    main()
