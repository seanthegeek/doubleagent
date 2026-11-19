"""Build the cursor package's test databases.

Run it from any directory:

    python3 -I internal/parser/cursor/testdata/fixtures.py

Each database below is a state.vscdb written by SQLite itself from the rows given, so the Go
tests read files SQLite wrote. Pages are 512 bytes to keep the files small. No test runs this
script; rerun it after changing the rows.
"""

from __future__ import annotations

import base64
import contextlib
import json
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB);
CREATE TABLE cursorDiskKV (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB);
CREATE TABLE composerHeaders (composerId TEXT PRIMARY KEY, workspaceId TEXT, createdAt INTEGER,
    lastUpdatedAt INTEGER, isArchived INTEGER, isSubagent INTEGER, recency INTEGER,
    checkpointAt INTEGER, subagentTypeName TEXT, value TEXT);
"""


def composer(ident: str) -> tuple[str, str]:
    """Return the composerData row of a composer whose header list is in chunks."""
    data = {"composerId": ident, "fullConversationHeadersOnly": [], "status": "completed"}
    return f"composerData:{ident}", json.dumps(data)


def chunk(ident: str, *bubbles: str) -> tuple[str, str]:
    """Return a composerConvHeaders chunk listing the bubble ids given."""
    entries = [{"bubbleId": bubble, "type": 1} for bubble in bubbles]
    return f"composerConvHeaders:{ident}:000000", json.dumps(entries)


def empty_composers(count: int) -> list[tuple[str, str]]:
    """Return count composers with an empty header list and ten bubbles each."""
    rows = []
    for number in range(count):
        ident = f"e{number:06d}"
        rows.append(composer(ident))
        for bubble in range(10):
            rows.append((f"bubbleId:{ident}:b{bubble}", json.dumps({"type": 1, "text": "x"})))
    return rows


def chunked() -> list[tuple[str, str]]:
    """Return composers a and a:b, whose chunk keys share a prefix, and c, with none."""
    return [
        composer("a"),
        composer("a:b"),
        composer("c"),
        chunk("a", "a1"),
        chunk("a:b", "ab1"),
    ]


def stray_chunks() -> list[tuple[str, str]]:
    """Return composer a with its chunk, and two chunk keys whose number is not digits."""
    return [
        composer("a"),
        chunk("a", "a1"),
        ("composerConvHeaders:a:b", json.dumps([{"bubbleId": "x1", "type": 1}])),
        ("composerConvHeaders:a:", json.dumps([{"bubbleId": "x2", "type": 1}])),
    ]


# A header with an empty composer id, archived as a delete leaves it, and a
# JSON agentKv blob naming no composer.
EMPTY_HEADER = """
INSERT INTO composerHeaders (composerId, isArchived, value) VALUES ('', 1, '{"isArchived": true}');
INSERT INTO cursorDiskKV VALUES ('agentKv:blob:00',
    '{"role": "user", "content": "a message of another conversation"}');
"""


# Headers whose value is a list, a string, JSON null and an empty object.
HEADER_KINDS = """
INSERT INTO composerHeaders (composerId, value) VALUES ('c1', '[1]');
INSERT INTO composerHeaders (composerId, value) VALUES ('c2', '"x"');
INSERT INTO composerHeaders (composerId, value) VALUES ('c3', 'null');
INSERT INTO composerHeaders (composerId, value) VALUES ('c4', '{}');
"""


def mcp_key(prefix: str, scope: str, field: str) -> str:
    """Return an MCP OAuth key: the prefix, then URL-safe base64 of "[scope] field"."""
    name = base64.urlsafe_b64encode(f"[{scope}] {field}".encode()).decode().rstrip("=")
    return prefix + name


# An MCP server's URL, which carries a credential, and its stored tokens, whose value is
# ciphertext.
MCP_SECRET = f"""
INSERT INTO ItemTable VALUES ('{mcp_key("mcpOAuth.secret.", "srv", "mcp_tokens")}', 'ciphertext');
INSERT INTO ItemTable VALUES ('{mcp_key("mcpOAuth.global.", "srv", "mcp_server_url")}',
    '"https://tok:s3cret@mcp.example.invalid/"');
"""

# A cloud agent whose repository URL carries a credential, the MCP authorisation answer that
# joins it on the agent's id, and the repository list naming the same URL.
CLOUD_REPOSITORY = """
INSERT INTO ItemTable VALUES ('cloudAgentRepository.agents.acct', '[{"bcId": "bc-1", "name": "fix",
    "repoUrl": "https://ci:s3cret@github.com/o/r", "status": "done", "createdAt": 1790845200000,
    "rowUpdatedAtMs": 1790845300000}]');
INSERT INTO ItemTable VALUES ('cloudAgentRepository.mcpAuthResolution',
    '[{"agentId": "bc-1", "toolCallId": "t1", "userDecision": "approved"}]');
INSERT INTO ItemTable VALUES ('cloudEnvironmentTemplate.githubRepos',
    '[{"url": "https://ci:s3cret@github.com/o/r", "type": "github"}]');
"""

DATABASES = {
    "empty_composers_2.db": (empty_composers(2), ""),
    "empty_composers_8.db": (empty_composers(8), ""),
    "chunked_headers.db": (chunked(), ""),
    "stray_chunks.db": (stray_chunks(), ""),
    "empty_composer_id.db": ([], EMPTY_HEADER),
    "mcp_secret.db": ([], MCP_SECRET),
    "header_kinds.db": ([], HEADER_KINDS),
    "cloud_repository.db": ([], CLOUD_REPOSITORY),
}


def main() -> None:
    """Rewrite every database of DATABASES in this directory."""
    for name, (rows, script) in DATABASES.items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript(SCHEMA + script)
            connection.executemany("INSERT INTO cursorDiskKV VALUES (?, ?)", rows)
            connection.commit()


if __name__ == "__main__":
    main()
