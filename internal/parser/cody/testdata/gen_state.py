"""Write the editor state databases the cody package's tests read.

Run from this directory with any Python 3: `python3 -I gen_state.py`.
Each holds an ItemTable whose sourcegraph.cody-ai row is:

- history-list.vscdb: an object whose chat history is a list.
- history-text.vscdb: an object whose chat history is text holding a list.
- value-number.vscdb: a number.
- no-history.vscdb: an object with model preferences and no chat history.
- history-null.vscdb: an object whose chat history is null.
- history-text-null.vscdb: an object whose chat history is text holding null.
- preferences-string.vscdb, preferences-number.vscdb, preferences-list.vscdb:
  an object whose model preferences are a string, a number and a list.
"""

import json
import os
import sqlite3

DATABASES = {
    "history-list.vscdb": {"cody-local-chatHistory-v2": [1]},
    "history-text.vscdb": {"cody-local-chatHistory-v2": "[1]"},
    "value-number.vscdb": 5,
    "no-history.vscdb": {"cody-model-preferences": {}},
    "history-null.vscdb": {"cody-local-chatHistory-v2": None},
    "history-text-null.vscdb": {"cody-local-chatHistory-v2": "null"},
    "preferences-string.vscdb": {"cody-model-preferences": "x"},
    "preferences-number.vscdb": {"cody-model-preferences": 7},
    "preferences-list.vscdb": {"cody-model-preferences": [1]},
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
            "INSERT INTO ItemTable VALUES (?, ?)", ("sourcegraph.cody-ai", json.dumps(value))
        )
        connection.commit()
        connection.execute("VACUUM")
        connection.close()


if __name__ == "__main__":
    main()
