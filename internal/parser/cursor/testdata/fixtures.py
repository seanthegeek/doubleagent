"""Build the cursor package's test databases.

Run it from any directory:

    python3 -I internal/parser/cursor/testdata/fixtures.py

Each database of DATABASES is a state.vscdb, and each of SEARCH_DATABASES a
conversation-search.db, written by SQLite itself from the rows given, so the Go tests read files
SQLite wrote. Pages are 512 bytes to keep the files small. No test runs this
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


def deleted_header(ident: str) -> str:
    """Return the SQL of the header a delete leaves: archived, with no composerData."""
    return f"INSERT INTO composerHeaders (composerId, isArchived, value) VALUES ('{ident}', 1, '{{}}');\n"


def blob(key: str, role: str, content: str) -> str:
    """Return the SQL of a JSON agentKv blob of a role and its content."""
    value = json.dumps({"role": role, "content": content})
    return f"INSERT INTO cursorDiskKV VALUES ('agentKv:blob:{key}', '{value}');\n"


def blob_fanout(count: int) -> str:
    """Return count deleted composers and one blob whose content names them all."""
    idents = [f"d{number:06d}" for number in range(count)]
    return "".join(deleted_header(ident) for ident in idents) + blob("00", "user", " ".join(idents))


# Three deleted composers; a user blob naming d2 and d3, a system blob naming d1 and d3, and
# an assistant blob naming d1 alone.
SHARED_BLOBS = (
    deleted_header("d1")
    + deleted_header("d2")
    + deleted_header("d3")
    + blob("01", "user", "asked in d3 and d2")
    + blob("02", "system", "prompt for d3 and d1")
    + blob("03", "assistant", "answer in d1")
)


def repeated_bubble(count: int) -> list[tuple[str, str]]:
    """Return composer c1 whose header list names bubble b count times, b's text 10 bytes each."""
    entries = [{"bubbleId": "b", "type": 2}] * count
    data = {"composerId": "c1", "fullConversationHeadersOnly": entries, "status": "completed"}
    return [
        ("composerData:c1", json.dumps(data)),
        ("bubbleId:c1:b", json.dumps({"type": 2, "text": "x" * (10 * count)})),
    ]


def repeated_bubbles() -> list[tuple[str, str]]:
    """Return composer c1 whose header list names b1, b2, b1, m, b1, b2 and m, m having no row."""
    order = ["b1", "b2", "b1", "m", "b1", "b2", "m"]
    entries = [{"bubbleId": bubble, "type": 2} for bubble in order]
    data = {"composerId": "c1", "fullConversationHeadersOnly": entries, "status": "completed"}
    return [
        ("composerData:c1", json.dumps(data)),
        ("bubbleId:c1:b1", json.dumps({"type": 2, "text": "first"})),
        ("bubbleId:c1:b2", json.dumps({"type": 2, "text": "second"})),
    ]


# ItemTable lists holding entries of the wrong kind: an MCP authorisation list with a number
# and a string beside its answer, a usages list with a number beside its usage, and a usages
# value that is an object.
ITEM_KINDS = """
INSERT INTO ItemTable VALUES ('cloudAgentRepository.mcpAuthResolution',
    '[5, "x", {"agentId": "bc-1", "toolCallId": "t1", "userDecision": "allow"}]');
INSERT INTO ItemTable VALUES ('github-alice', '[{"id": "e", "name": "E", "allowed": true}]');
INSERT INTO ItemTable VALUES ('github-alice-usages', '{"not": "a list"}');
INSERT INTO ItemTable VALUES ('github-bob', '[{"id": "e", "name": "E", "allowed": true}]');
INSERT INTO ItemTable VALUES ('github-bob-usages', '[1, {"lastUsed": 1790845200000, "scopes": ["repo"]}]');
"""

# An MCP authorisation value that is neither a list nor an answer.
MCP_AUTH_TEXT = """
INSERT INTO ItemTable VALUES ('cloudAgentRepository.mcpAuthResolution', '"allow"');
"""

# Plugin scopes whose file: path holds a URL with credentials: in a drive path, in a drive path
# percent-encoded, in a network path and in a POSIX path.
PLUGIN_SCOPES = "".join(
    f"INSERT INTO ItemTable VALUES ('cursor.plugins.installedIds.team|{scope}',"
    """ '[{"id": "p1", "sources": ["s"]}]');\n"""
    for scope in (
        "file:///c:/w/https://ef:gh@example.org/r",
        "file:///c%3A/w/https%3A%2F%2Fab%3Acd%40example.com%2Fr",
        "file://server/https://ij:kl@example.net/r",
        "file:///srv/https://mn:op@example.com/r",
    )
)

DATABASES = {
    "empty_composers_2.db": (empty_composers(2), ""),
    "empty_composers_8.db": (empty_composers(8), ""),
    "chunked_headers.db": (chunked(), ""),
    "stray_chunks.db": (stray_chunks(), ""),
    "empty_composer_id.db": ([], EMPTY_HEADER),
    "mcp_secret.db": ([], MCP_SECRET),
    "header_kinds.db": ([], HEADER_KINDS),
    "cloud_repository.db": ([], CLOUD_REPOSITORY),
    "blob_fanout_20.db": ([], blob_fanout(20)),
    "blob_fanout_40.db": ([], blob_fanout(40)),
    "shared_blobs.db": ([], SHARED_BLOBS),
    "repeated_bubble_20.db": (repeated_bubble(20), ""),
    "repeated_bubble_40.db": (repeated_bubble(40), ""),
    "repeated_bubbles.db": (repeated_bubbles(), ""),
    "item_kinds.db": ([], ITEM_KINDS),
    "mcp_auth_text.db": ([], MCP_AUTH_TEXT),
    "plugin_scopes.db": ([], PLUGIN_SCOPES),
}


SEARCH_SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE conversations (fts_rowid INTEGER, source TEXT, scope TEXT, id TEXT, title TEXT,
    branches TEXT, updated_at INTEGER, is_archived INTEGER);
CREATE TABLE conversation_fts_content (id INTEGER PRIMARY KEY, c0, c1, c2);
"""


def conversation(ident: str, fts_rowid: int) -> str:
    """Return the SQL of a local conversation-search.db conversation joined on fts_rowid."""
    return (
        f"INSERT INTO conversations VALUES ({fts_rowid}, 'local', '', '{ident}', 'Title {ident}', '',"
        " 1767323045000, 0);\n"
    )


def body(fts_rowid: int, text: str) -> str:
    """Return the SQL of a full-text content row whose body is text."""
    return f"INSERT INTO conversation_fts_content VALUES ({fts_rowid}, 'title', '{text}', '');\n"


def shared_body(count: int) -> str:
    """Return count conversations that all join one body of 10 bytes per conversation."""
    conversations = "".join(conversation(f"c{number}", 1) for number in range(count))
    return conversations + body(1, "y" * (10 * count))


# c1, c2 and c4 join full-text row 1, c3 joins row 2.
SHARED_BODIES = (
    conversation("c1", 1)
    + conversation("c2", 1)
    + conversation("c3", 2)
    + conversation("c4", 1)
    + body(1, "body one")
    + body(2, "body two")
)

SEARCH_DATABASES = {
    "shared_body_20.db": shared_body(20),
    "shared_body_40.db": shared_body(40),
    "shared_bodies.db": SHARED_BODIES,
}


def write(name: str, script: str, rows: list[tuple[str, str]]) -> None:
    """Rewrite the database name in this directory from a script and cursorDiskKV rows."""
    path = HERE / name
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(script)
        if rows:
            connection.executemany("INSERT INTO cursorDiskKV VALUES (?, ?)", rows)
        connection.commit()


def main() -> None:
    """Rewrite every database of DATABASES and SEARCH_DATABASES in this directory."""
    for name, (rows, script) in DATABASES.items():
        write(name, SCHEMA + script, rows)
    for name, script in SEARCH_DATABASES.items():
        write(name, SEARCH_SCHEMA + script, [])


if __name__ == "__main__":
    main()
