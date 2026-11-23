"""Write the editor state databases the tabby package's tests read.

Run from this directory with any Python 3: `python3 -I gen_state.py`.
Each holds an ItemTable whose TabbyML.vscode-tabby row is an object whose
edit.recentlyCommand member is, by file name:

- command-string.vscdb: a string.
- command-number.vscdb: a number.
- command-object.vscdb: an object.
- command-null.vscdb: null.
- command-empty.vscdb: an empty array.

server-records.vscdb holds a server.serverRecords object of one endpoint with a token beside
an empty history; servers-list.vscdb holds a server.serverRecords that is a list, and
servers-string.vscdb one whose endpoint holds a string in place of its record.
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
    "server-records.vscdb": {
        "server.serverRecords": {"https://ci:s3cret@tabby.example/": {"token": "auth_FIXTURE"}},
        "edit.recentlyCommand": [],
    },
    "servers-list.vscdb": {"server.serverRecords": ["https://tabby.example/"]},
    "servers-string.vscdb": {"server.serverRecords": {"https://tabby.example/": "auth_FIXTURE"}},
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
