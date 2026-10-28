#!/usr/bin/env python3
"""Build the synthetic collection of host `wsdev14` that the output examples are a run on.

A working day on the Linux host `wsdev14`, where alice and bob share a Go repository,
`/srv/proj`: alice's Claude Code terminal session that fixes a rounding bug, turns on Remote
Control and takes a prompt from her phone; a session Claude Code's Remote Control server spawned
for a prompt from claude.ai, with its `ccr-tip.json` and the project's `bridge-pointer.json`; a
cloud session she started with `claude --cloud` and teleported to the host; the registry and
peer-key files of her two running processes; her prompt history with its paste cache, her
settings and `.claude.json`; bob's Codex CLI session with its history, state database and
`config.toml`; his Gemini CLI settings and a chat the collection lost; his Ollama history and a
model manifest; alice's Amp settings; and the repository's git metadata and agent files.

Every prompt, reply, tool result, id and time is invented; the ids derive from `uuid5` values over
`NAMESPACE`, so they look like an agent's and never change. The record shapes follow the fixture
builders of `analyzer/tests/fixtures.py` (`claude_310_records`, `claude_mac_records`,
`claude_mac_spawn_records`, `claude_mac_teleport_records`, `claude_309_records`,
`claude_registry_records`, `codex_rollout_records`, `write_codex_state`) and the research documents
(`research/claude-code.md`, Record schema and Remote Control; `research/codex-cli.md`, Parser plan;
`research/gemini-cli.md` and `research/ollama.md`, Per-user storage). The manifest,
`collection.json` and `collector.log` are written by this module in the shape
`collectors/docs/output.md` documents for the sh collector, not by the collector: one manifest row
is cut short, and one collected file is left out of `fs/`. The credential files a signed-in install
holds (`.claude/.credentials.json`, `.codex/auth.json`) are left out on purpose, so no sign-in
token shape enters the repository.

    python3 analyzer/tools/example_collection.py <empty directory>

writes the collection to `<empty directory>/wsdev14` and prints that path. Exit status: 0
when it was written, 2 unless the one argument names an empty directory; a failed write raises.
"""

from __future__ import annotations

import hashlib
import json
import os
import shlex
import sqlite3
import sys
import tempfile
import uuid
from collections.abc import Iterator
from datetime import datetime, timezone
from pathlib import Path
from typing import NamedTuple

HOST = "wsdev14"
COLLECTOR_VERSION = "1.19.0"
NAMESPACE = uuid.UUID("6f1d2c4e-8a3b-5c7d-9e0f-1a2b3c4d5e6f")
UIDS = {"alice": 1000, "bob": 1001}
# The developers' shared group that owns `/srv/proj`, so both users write to it.
PROJECT_GID = 1002
PROJECT = "/srv/proj"
ALICE_HOME = "/home/alice"
BOB_HOME = "/home/bob"
CLAUDE_VERSION = "2.1.310"
CODEX_VERSION = "0.162.1"
# A cloud session needs a GitHub repository; GitHub registers no owner with two hyphens in a
# row, so this URL can never name a real one.
ORIGIN_URL = "git@github.com:acme--ledger/ledger.git"
UPSTREAM_URL = "https://git.example.com/acme-billing/ledger.git"
FIX_BRANCH = "fix/invoice-rounding"
OPUS = "claude-opus-5-5"
FABLE = "claude-fable-5-1"
CODEX_MODEL = "gpt-6.1-sol"


def make_uuid(name: str, version: int = 4) -> str:
    """Return the example's id for `name`: `uuid5` over `NAMESPACE`, restamped as `version`.

    Claude Code gives a session a random (version 4) id, and only the child a Remote Control
    server spawns a name-based (version 5) one, so the version digit is part of the shape.
    """
    value = uuid.uuid5(NAMESPACE, name).int
    return str(uuid.UUID(int=(value & ~(0xF << 76)) | (version << 76)))


def make_hex(name: str, length: int) -> str:
    """Return `length` hex digits of the `uuid5` ids of `name`, for the agents' hex ids."""
    digits = "".join(uuid.uuid5(NAMESPACE, "%s %d" % (name, part)).hex for part in range(2))
    return digits[:length]


def make_base62(name: str, length: int) -> str:
    """Return `length` letters and digits from the `uuid5` ids of `name`, for the API's ids."""
    alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    number = int(make_hex(name, 64), 16)
    letters = []
    for _ in range(length):
        number, digit = divmod(number, 62)
        letters.append(alphabet[digit])
    return "".join(letters)


# alice's sessions of the day: the terminal session, the Remote Control spawn, the teleported
# cloud session, and the two sessions only her history still names.
TERMINAL_SESSION = make_uuid("alice terminal session")
SPAWN_SESSION = make_uuid("alice remote control spawn", version=5)
TELEPORT_SESSION = make_uuid("alice teleported cloud session")
DELETED_SESSION = make_uuid("alice deleted session")
REMOVED_REPOSITORY_SESSION = make_uuid("alice invoicing-old session")
BRIDGE_HEX = make_hex("alice remote control bridge", 16)
BRIDGE_SESSION = "cse_" + BRIDGE_HEX
BRIDGE_URL = "https://claude.ai/code/session_" + BRIDGE_HEX
# alice's claude.ai account and organisation, named by the bridge records and the GitHub
# connection alike.
ALICE_ACCOUNT = make_uuid("alice account")
ACME_ORGANIZATION = make_uuid("acme organization")
TERMINAL_PID = 48213
TELEPORT_PID = 52904
FIX_COMMIT_FULL = make_hex("rounding fix commit", 40)
FIX_COMMIT = FIX_COMMIT_FULL[:7]
CLONE_COMMIT = make_hex("clone head commit", 40)
MAIN_COMMIT = make_hex("main head commit", 40)
BOB_COMMIT = make_hex("csv export fix commit", 40)
PASTE_HASH = make_hex("ci failure paste", 16)
GEMINI_SESSION = make_uuid("bob gemini chat")
# bob's older Gemini CLI build names the project directory by its hash and writes one-object chats.
GEMINI_CHAT = ".gemini/tmp/%s/chats/session-2026-10-27T09-41-%s.json" % (
    hashlib.sha256(PROJECT.encode("utf-8")).hexdigest(),
    GEMINI_SESSION[:8],
)


def stamp(clock: str) -> str:
    """Return the ISO 8601 UTC time of `clock` (`HH:MM:SS.mmm`) on the example's day."""
    return "2026-10-27T%sZ" % clock


def epoch(clock: str) -> float:
    """Return `clock` (`HH:MM:SS` or `HH:MM:SS.mmm`) on the example's day in epoch seconds."""
    moment = datetime.strptime(
        "2026-10-27 " + clock, "%Y-%m-%d %H:%M:%S.%f" if "." in clock else "%Y-%m-%d %H:%M:%S"
    )
    return moment.replace(tzinfo=timezone.utc).timestamp()


def epoch_ms(clock: str) -> int:
    """Return `clock` on the example's day in epoch milliseconds."""
    return round(epoch(clock) * 1000)


def make_uuid7(name: str, clock: str) -> str:
    """Return a time-ordered (version 7) id for `name` at `clock`, the form Codex CLI writes."""
    value = (
        (epoch_ms(clock) << 80) | (7 << 76) | (uuid.uuid5(NAMESPACE, name).int & ((1 << 76) - 1))
    )
    return str(uuid.UUID(int=(value & ~(0x3 << 62)) | (0x2 << 62)))


def start_tick(clock: str) -> str:
    """Return `procStart` for a process started at `clock`: its start tick since a midnight boot."""
    return str(round((epoch(clock) - epoch("00:00:00")) * 100))


SHELL_RESULT = {"interrupted": False, "isImage": False, "noOutputExpected": False, "type": "text"}
ONCE = {
    "decision": "accept",
    "source": "user_temporary",
    "grantScope": "once",
    "reasonCode": "bashMiss:no-rule-match",
}
ALWAYS = {"decision": "accept", "source": "user_permanent", "reasonCode": "bashMiss:no-rule-match"}
BY_MODE = {"decision": "accept", "source": "config", "reasonType": "mode"}
READ_ONLY = {"decision": "accept", "source": "config", "reasonType": "subcommandResults"}
REFUSED = {"decision": "reject", "source": "user_reject", "reasonCode": "bashMiss:no-rule-match"}
REJECTED_TEXT = "The user doesn't want to proceed with this tool use."


class _Transcript:
    """A Claude Code session file being written, record by record, in the 2.1.310 key set."""

    def __init__(self, session_id: str, branch: str, entrypoint: str = "cli") -> None:
        self.session_id = session_id
        self.common = {
            "userType": "external",
            "entrypoint": entrypoint,
            "cwd": PROJECT,
            "sessionId": session_id,
            "version": CLAUDE_VERSION,
            "gitBranch": branch,
        }
        self.records: list[dict] = []
        self.parent: str | None = None
        self.count = 0

    def state(self, record_type: str, **keys) -> None:
        self.records.append(dict(type=record_type, **keys, sessionId=self.session_id))

    def record(self, record_type: str, clock: str, **keys) -> dict:
        self.count += 1
        record_id = make_uuid("%s record %d" % (self.session_id, self.count))
        record = {"parentUuid": self.parent, "isSidechain": False, "type": record_type}
        record.update(keys)
        record.update(uuid=record_id, timestamp=stamp(clock), **self.common)
        self.parent = record_id
        self.records.append(record)
        return record

    def prompt(self, clock: str, text: str, mode: str, source: str = "typed", **keys) -> None:
        self.record(
            "user",
            clock,
            message={"role": "user", "content": text},
            origin={"kind": "human"},
            promptSource=source,
            turnOrigin="human",
            permissionMode=mode,
            **keys,
        )

    def assistant(self, clock: str, model: str, message: str, block: dict, **keys) -> None:
        body: dict = {"model": model, "id": "msg_01" + make_base62(message, 22), "type": "message"}
        body.update(role="assistant", content=[block])
        request = "req_011C" + make_base62(message + " request", 20)
        self.record(
            "assistant", clock, message=body, requestId=request, requestedModel=model, **keys
        )

    def text(self, clock: str, model: str, message: str, text: str, **keys) -> None:
        self.assistant(clock, model, message, {"type": "text", "text": text}, **keys)

    def call(self, clock: str, model: str, message: str, name: str, tool_input: dict) -> str:
        tool_id = "toolu_01" + make_base62("%s %s tool" % (self.session_id, message), 22)
        block = {"type": "tool_use", "id": tool_id, "name": name, "input": tool_input}
        self.assistant(clock, model, message, block)
        return tool_id

    def result(self, clock: str, tool_id: str, content: str, decision: dict | None, **keys):
        error = content.startswith("Exit code ") or content == REJECTED_TEXT
        block: dict = {"tool_use_id": tool_id, "type": "tool_result", "content": content}
        block["is_error"] = error
        if decision is not None:
            keys["permissionDecision"] = decision
        self.record(
            "user",
            clock,
            message={"role": "user", "content": [block]},
            sourceToolAssistantUUID=self.parent,
            **keys,
        )

    def shell(self, clock: str, tool_id: str, output: str, decision: dict, exit_code: int = 0):
        content = output if exit_code == 0 else "Exit code %d\n%s" % (exit_code, output)
        stdout = dict(SHELL_RESULT, stdout=output, stderr="")
        self.result(clock, tool_id, content, decision, sandboxed=False, toolUseResult=stdout)

    def environment(self, clock: str, scratchpad: str, working_directory: str = PROJECT) -> None:
        snapshot = {
            "additionalWorkingDirectories": [],
            "isGitRepo": True,
            "isWorktree": False,
            "osVersion": "Linux 6.8.0-45-generic",
            "platform": "linux",
            "scratchpadDirectory": scratchpad,
            "shell": "bash",
            "workingDirectory": working_directory,
        }
        self.record("attachment", clock, attachment={"type": "environment", "snapshot": snapshot})

    def system(self, clock: str, subtype: str, level: str = "info", **keys) -> None:
        self.record("system", clock, subtype=subtype, **keys, isMeta=False, level=level)


def slash_command(name: str, args: str = "") -> str:
    """Return a slash command's `user` text in the transcript's tag form."""
    return (
        "<command-name>/%s</command-name>\n            <command-message>%s</command-message>\n"
        "            <command-args>%s</command-args>" % (name, name, args)
    )


# Longer than the 1,024 characters above which Claude Code keeps a paste in `paste-cache/`.
CI_FAILURE = """\
=== RUN   TestInvoiceTotal
=== RUN   TestInvoiceTotal/single_line
=== RUN   TestInvoiceTotal/quantity_three
--- PASS: TestInvoiceTotal (0.00s)
    --- PASS: TestInvoiceTotal/single_line (0.00s)
    --- PASS: TestInvoiceTotal/quantity_three (0.00s)
=== RUN   TestRoundCents
=== RUN   TestRoundCents/whole_dollars
=== RUN   TestRoundCents/half_cent_up
=== RUN   TestRoundCents/half_cent_down
=== RUN   TestRoundCents/negative_refund
=== RUN   TestRoundCents/zero
--- FAIL: TestRoundCents (0.00s)
    --- PASS: TestRoundCents/whole_dollars (0.00s)
    --- FAIL: TestRoundCents/half_cent_up (0.00s)
        round_test.go:31: RoundCents(1.005) = 100, want 101
    --- PASS: TestRoundCents/half_cent_down (0.00s)
    --- FAIL: TestRoundCents/negative_refund (0.00s)
        round_test.go:31: RoundCents(-1.005) = -100, want -101
    --- PASS: TestRoundCents/zero (0.00s)
=== RUN   TestParseAmount
=== RUN   TestParseAmount/plain
=== RUN   TestParseAmount/thousands_separator
=== RUN   TestParseAmount/negative
--- PASS: TestParseAmount (0.00s)
    --- PASS: TestParseAmount/plain (0.00s)
    --- PASS: TestParseAmount/thousands_separator (0.00s)
    --- PASS: TestParseAmount/negative (0.00s)
FAIL
FAIL\tgithub.com/acme--ledger/ledger/internal/invoice\t0.014s"""
FIRST_PROMPT = (
    "CI has failed like this on main since this morning's merge. Find the cause and fix it "
    "without changing the test expectations."
)
COMMIT_PROMPT = (
    "Commit this on the branch with a message that explains the float problem, then push it."
)
PHONE_PROMPT = "What's the hash of the rounding fix commit?"
GREP_OUTPUT = """\
internal/invoice/round.go:7:func RoundCents(amount float64) int64 {
internal/invoice/total.go:18:\t\tsum += RoundCents(line.Amount) * int64(line.Quantity)
internal/invoice/round_test.go:24:\t\tgot := RoundCents(tc.amount)"""
ROUND_SOURCE = """\
     1\tpackage invoice
     2\t
     3\timport "math"
     4\t
     5\t// RoundCents converts an amount in dollars to whole cents,
     6\t// rounding half away from zero.
     7\tfunc RoundCents(amount float64) int64 {
     8\t\treturn int64(math.Round(amount * 100))
     9\t}"""
FIRST_EDIT = {
    "file_path": "/srv/proj/internal/invoice/round.go",
    "old_string": "\treturn int64(math.Round(amount * 100))",
    "new_string": "\tmills := int64(math.Round(amount * 1000))\n\treturn (mills + 5) / 10",
    "replace_all": False,
}
SECOND_EDIT = {
    "file_path": "/srv/proj/internal/invoice/round.go",
    "old_string": "\treturn (mills + 5) / 10",
    "new_string": "\tif mills < 0 {\n\t\treturn (mills - 5) / 10\n\t}\n\treturn (mills + 5) / 10",
    "replace_all": False,
}
EDITED = (
    "The file /srv/proj/internal/invoice/round.go has been updated. Here's the result of "
    "running `cat -n` on a snippet of the edited file:\n%s"
)
FIRST_TEST_RUN = """\
go vet ./...
go test ./...
ok  \tgithub.com/acme--ledger/ledger/cmd/ledger\t0.019s
ok  \tgithub.com/acme--ledger/ledger/internal/api\t0.176s
--- FAIL: TestRoundCents (0.00s)
    --- FAIL: TestRoundCents/negative_refund (0.00s)
        round_test.go:31: RoundCents(-1.005) = -100, want -101
FAIL
FAIL\tgithub.com/acme--ledger/ledger/internal/invoice\t0.011s
ok  \tgithub.com/acme--ledger/ledger/internal/store\t0.298s
FAIL
make: *** [Makefile:14: test] Error 1"""
SECOND_TEST_RUN = """\
go vet ./...
go test ./...
ok  \tgithub.com/acme--ledger/ledger/cmd/ledger\t0.018s
ok  \tgithub.com/acme--ledger/ledger/internal/api\t0.181s
ok  \tgithub.com/acme--ledger/ledger/internal/invoice\t0.010s
ok  \tgithub.com/acme--ledger/ledger/internal/store\t0.305s"""
GIT_STATUS = """\
On branch fix/invoice-rounding
Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
\tmodified:   internal/invoice/round.go

no changes added to commit (use "git add" and/or "git commit -a")"""
COMMIT_COMMAND = (
    'git commit -am "Round invoice cents from thousandths" -m "1.005 * 100 is '
    "100.49999999999999 in float64, so math.Round gave 100. Round to thousandths first and "
    'add or subtract the half cent in integer arithmetic."'
)


def claude_terminal_records() -> list[dict]:
    """Return alice's morning terminal session: the rounding fix, `/model` and Remote Control."""
    session = _Transcript(TERMINAL_SESSION, FIX_BRANCH)
    scratchpad = "/tmp/claude-1000/-srv-proj/%s/scratchpad" % TERMINAL_SESSION
    session.state("mode", mode="normal")
    session.state("permission-mode", permissionMode="acceptEdits")
    paste = '<pasted_content id="%s">\n%s\n</pasted_content> ' % (PASTE_HASH[:4], CI_FAILURE)
    session.prompt(
        "09:12:03.087",
        paste + FIRST_PROMPT,
        "acceptEdits",
        turnPosition={"promptIndex": 0, "turnIndex": 0},
    )
    session.environment("09:12:03.091", scratchpad)
    session.text(
        "09:12:08.412",
        OPUS,
        "s1 grep",
        "Both failures are in RoundCents. I'll find where it is defined and who calls it.",
    )
    tool_id = session.call(
        "09:12:08.977",
        OPUS,
        "s1 grep",
        "Grep",
        {"pattern": "RoundCents", "path": PROJECT, "output_mode": "content", "-n": True},
    )
    session.result("09:12:09.204", tool_id, GREP_OUTPUT, BY_MODE)
    read = {"file_path": "/srv/proj/internal/invoice/round.go"}
    tool_id = session.call("09:12:12.530", OPUS, "s1 read", "Read", read)
    session.result("09:12:12.611", tool_id, ROUND_SOURCE, BY_MODE)
    session.text(
        "09:12:24.019",
        OPUS,
        "s1 first edit",
        "The cause is float rounding: 1.005 * 100 is 100.49999999999999 in float64, so "
        "math.Round returns 100. I'll round to thousandths first and add the half cent in "
        "integer arithmetic, which is exact.",
    )
    tool_id = session.call("09:12:25.330", OPUS, "s1 first edit", "Edit", FIRST_EDIT)
    snippet = (
        "     7\tfunc RoundCents(amount float64) int64 {\n"
        "     8\t\tmills := int64(math.Round(amount * 1000))\n"
        "     9\t\treturn (mills + 5) / 10\n    10\t}"
    )
    session.result("09:12:25.391", tool_id, EDITED % snippet, BY_MODE)
    make_test = {"command": "make test", "description": "Run the unit tests"}
    tool_id = session.call("09:12:28.804", OPUS, "s1 first test", "Bash", make_test)
    session.shell("09:12:41.115", tool_id, FIRST_TEST_RUN, ONCE, exit_code=2)
    session.text(
        "09:12:47.302",
        OPUS,
        "s1 second edit",
        "The positive case passes now, but adding the half cent moves a negative amount toward "
        "zero: -1005 thousandths becomes -1000, so -100. Negative amounts need it subtracted.",
    )
    tool_id = session.call("09:12:48.160", OPUS, "s1 second edit", "Edit", SECOND_EDIT)
    snippet = (
        "     8\t\tmills := int64(math.Round(amount * 1000))\n     9\t\tif mills < 0 {\n"
        "    10\t\t\treturn (mills - 5) / 10\n    11\t\t}\n    12\t\treturn (mills + 5) / 10"
    )
    session.result("09:12:48.214", tool_id, EDITED % snippet, BY_MODE)
    tool_id = session.call("09:12:51.067", OPUS, "s1 second test", "Bash", make_test)
    session.shell("09:13:19.842", tool_id, SECOND_TEST_RUN, ALWAYS)
    status = {"command": "git status", "description": "Show the working tree status"}
    tool_id = session.call("09:13:23.509", OPUS, "s1 status", "Bash", status)
    session.shell("09:13:23.618", tool_id, GIT_STATUS, READ_ONLY)
    session.text(
        "09:13:30.271",
        OPUS,
        "s1 summary",
        "All tests pass. RoundCents now rounds half away from zero from thousandths, which "
        "gives 101 for 1.005 and -101 for -1.005; only round.go changed.",
    )
    session.system("09:13:30.288", "turn_duration", durationMs=87201, messageCount=21)
    session.state("ai-title", aiTitle="Fix invoice cent rounding")
    session.prompt("09:24:40.551", slash_command("model"), "acceptEdits", isMeta=False)
    stdout = (
        "<local-command-stdout>Set model to `Fable 5.1` and saved as your default for new "
        "sessions</local-command-stdout>"
    )
    session.record("user", "09:24:40.552", message={"role": "user", "content": stdout})
    session.prompt("09:25:02.904", COMMIT_PROMPT, "acceptEdits")
    session.text(
        "09:25:09.633",
        FABLE,
        "s1 commit",
        "I'll commit round.go with the float explanation in the body, then push the branch.",
    )
    commit = {"command": COMMIT_COMMAND, "description": "Commit the rounding fix"}
    tool_id = session.call("09:25:10.118", FABLE, "s1 commit", "Bash", commit)
    committed = (
        "[%s %s] Round invoice cents from thousandths\n 1 file changed, 5 insertions(+), "
        "1 deletion(-)" % (FIX_BRANCH, FIX_COMMIT)
    )
    session.shell("09:25:15.447", tool_id, committed, ONCE)
    push = {"command": "git push -u origin " + FIX_BRANCH, "description": "Push the branch"}
    tool_id = session.call("09:25:18.026", FABLE, "s1 push", "Bash", push)
    session.result(
        "09:25:31.790",
        tool_id,
        REJECTED_TEXT,
        REFUSED,
        toolUseResult="User rejected tool use",
        toolDenialKind="user-rejected",
        userFeedback="Don't push yet; I'll push it myself after the review meeting.",
    )
    session.text(
        "09:25:36.412",
        FABLE,
        "s1 not pushed",
        "Understood: the commit stays local on %s and nothing was pushed. Run `git push -u "
        "origin %s` when you are ready." % (FIX_BRANCH, FIX_BRANCH),
    )
    session.system("09:25:36.430", "turn_duration", durationMs=33526, messageCount=8)
    session.system(
        "09:31:10.205",
        "local_command",
        commandRun="remote-control",
        content=slash_command("remote-control"),
    )
    session.system(
        "09:31:12.660", "bridge_status", content="/remote-control is active", url=BRIDGE_URL
    )
    _bridge(session, 1)
    enqueue = {"operation": "enqueue", "content": PHONE_PROMPT, "timestamp": stamp("11:02:47.318")}
    session.state("queue-operation", **enqueue)
    session.state("queue-operation", operation="dequeue", timestamp=stamp("11:02:47.320"))
    session.prompt(
        "11:02:47.341", PHONE_PROMPT, "acceptEdits", source="queued", queueSkipSlashCommands=True
    )
    _bridge(session, 4)
    log = {"command": "git log -1 --oneline", "description": "Show the last commit"}
    tool_id = session.call("11:02:52.774", FABLE, "s1 log", "Bash", log)
    session.shell(
        "11:02:52.903", tool_id, FIX_COMMIT + " Round invoice cents from thousandths", READ_ONLY
    )
    session.text(
        "11:02:57.115",
        FABLE,
        "s1 hash",
        "The fix is commit %s on %s. It is still only on this machine; nothing has been "
        "pushed." % (FIX_COMMIT, FIX_BRANCH),
    )
    session.system("11:02:57.130", "turn_duration", durationMs=9789, messageCount=5)
    return session.records


def _bridge(session: _Transcript, sequence: int) -> None:
    session.records.append(
        {
            "type": "bridge-session",
            "bridgeSessionId": BRIDGE_SESSION,
            "lastSequenceNum": sequence,
            "ownerAccountUuid": ALICE_ACCOUNT,
            "ownerOrganizationUuid": ACME_ORGANIZATION,
            "sessionId": session.session_id,
        }
    )


SPAWN_PROMPT = (
    "I pulled main at lunch. Run the whole test suite and tell me whether anything outside the "
    "invoice package fails."
)
# `internal/export` reached `main` with the 13:05 pull; the morning runs on the clone had no such
# package, so only the afternoon runs list it.
SUITE_RUN = """\
ok  \tgithub.com/acme--ledger/ledger/cmd/ledger\t0.021s
ok  \tgithub.com/acme--ledger/ledger/internal/api\t0.184s
--- FAIL: TestExportCSV_Amounts (0.00s)
    csv_test.go:42: row 2 amount = "12.50 ", want "12.50"
FAIL
FAIL\tgithub.com/acme--ledger/ledger/internal/export\t0.009s
--- FAIL: TestRoundCents (0.00s)
    --- FAIL: TestRoundCents/half_cent_up (0.00s)
        round_test.go:31: RoundCents(1.005) = 100, want 101
    --- FAIL: TestRoundCents/negative_refund (0.00s)
        round_test.go:31: RoundCents(-1.005) = -100, want -101
FAIL
FAIL\tgithub.com/acme--ledger/ledger/internal/invoice\t0.012s
ok  \tgithub.com/acme--ledger/ledger/internal/store\t0.311s
FAIL"""
BY_RULE = {"decision": "accept", "source": "config", "reasonType": "rule"}


def claude_remote_control_records() -> list[dict]:
    """Return the session Remote Control's server spawned for a prompt alice sent from claude.ai."""
    session = _Transcript(SPAWN_SESSION, "main", entrypoint="sdk-cli")
    enqueue = {"operation": "enqueue", "content": SPAWN_PROMPT, "timestamp": stamp("13:31:05.512")}
    session.state("queue-operation", **enqueue)
    session.state("queue-operation", operation="dequeue", timestamp=stamp("13:31:05.515"))
    # A spawned session's prompt records `default` rather than the user's `defaultMode`
    # (research/claude-code.md, Remote Control).
    session.prompt("13:31:05.604", SPAWN_PROMPT, "default", source="sdk")
    session.text(
        "13:31:10.877",
        FABLE,
        "s2 suite",
        "I'll run the whole suite with go test from the repository root.",
    )
    suite = {"command": "go test ./...", "description": "Run every package's tests"}
    tool_id = session.call("13:31:11.240", FABLE, "s2 suite", "Bash", suite)
    decision = dict(BY_RULE, ruleSource="userSettings")
    session.shell("13:31:39.065", tool_id, SUITE_RUN, decision, exit_code=1)
    session.text(
        "13:31:47.392",
        FABLE,
        "s2 answer",
        "One package outside invoice fails: internal/export, where TestExportCSV_Amounts finds "
        'a trailing space in "12.50 ". invoice still fails as it did on main this morning, and '
        "every other package passes.",
    )
    session.state(
        "cost-state",
        hasUnknownModelCost=False,
        modelUsage={},
        startTime=epoch_ms("13:31:05.512"),
        totalAPIDuration=11840,
        totalAPIDurationWithoutRetries=11840,
        totalCostUSD=0.04,
        totalDuration=41880,
        totalLinesAdded=0,
        totalLinesRemoved=0,
        totalToolDuration=27825,
    )
    return session.records


README_SOURCE = """\
     1\t# ledger
     2\t
     3\tThe billing team's invoice service: it records invoices and exports them as CSV.
     4\t
     5\t## Make targets
     6\t
     7\t- `make build` compiles the service into `bin/ledger`.
     8\t- `make test` runs `go vet` and the unit tests.
     9\t- `make lint` runs gofmt and the linter.
    10\t- `make migrate` applies the migrations in `migrations/`."""
CLOUD_PROMPT = "Read the README and list the make targets it documents, one line each."
LOCAL_PROMPT = "Which of those should I run before I push?"


def claude_teleport_records() -> list[dict]:
    """Return the cloud session alice started with `claude --cloud` and teleported to the host."""
    session = _Transcript(TELEPORT_SESSION, "main")
    cloud = "/tmp/claude-0/-home-user-repo/%s/scratchpad" % make_uuid("cloud machine session")
    remote = {"remoteSourced": True}
    session.environment("16:40:02.118", cloud, working_directory="/home/user/repo")
    session.prompt("16:40:02.250", CLOUD_PROMPT, "default", source="sdk", **remote)
    read = {"file_path": "/home/user/repo/README.md"}
    tool_id = "toolu_01" + make_base62("s3 cloud read tool", 22)
    block = {"type": "tool_use", "id": tool_id, "name": "Read", "input": read}
    session.assistant("16:40:06.913", OPUS, "s3 read", block, **remote)
    decision = dict(BY_RULE, ruleSource="cliArg")
    session.result("16:40:07.020", tool_id, README_SOURCE, decision, **remote)
    session.text(
        "16:40:13.587",
        OPUS,
        "s3 targets",
        "The README documents four targets: `make build` compiles bin/ledger, `make test` runs "
        "go vet and the unit tests, `make lint` runs gofmt and the linter, and `make migrate` "
        "applies the migrations.",
        **remote,
    )
    session.system(
        "16:40:13.640",
        "stop_hook_summary",
        hookCount=1,
        hookErrors=[],
        hookInfos=[],
        hookAdditionalContext=[],
        hasOutput=False,
        preventedContinuation=False,
        stopReason="",
        toolUseID="",
    )
    session.system("16:55:41.006", "informational", "notice", content="Session resumed")
    session.environment(
        "16:55:41.212", "/tmp/claude-1000/-srv-proj/%s/scratchpad" % TELEPORT_SESSION
    )
    session.prompt("16:56:02.377", LOCAL_PROMPT, "acceptEdits")
    session.text(
        "16:56:08.940",
        FABLE,
        "s3 answer",
        "Run `make test`, which AGENTS.md asks for before every commit, and `make lint`, which "
        "it wants clean. `make migrate` is only needed when you add a migration.",
    )
    return session.records


# The turn's closing record, as the copy caught it while Claude Code was still writing it.
TELEPORT_CUT_LINE = (
    '{"parentUuid":"%s","isSidechain":false,"type":"system","subtype":"turn_duration",'
    '"durationMs":6563,"messageCount":3,"isMeta":false,"level":"info","uuid":"'
)


def _jsonl(records: list[dict]) -> str:
    return "".join(
        json.dumps(record, ensure_ascii=False, separators=(",", ":")) + "\n" for record in records
    )


def teleport_transcript() -> str:
    """Return the teleported session's file: its records, then the last one cut mid-write."""
    records = claude_teleport_records()
    return _jsonl(records) + TELEPORT_CUT_LINE % records[-1]["uuid"]


def claude_history_records() -> list[dict]:
    """Return alice's `history.jsonl`: her typed prompts and commands, oldest first."""

    def line(clock: str, display: str, project: str, session_id: str, **pasted) -> dict:
        return {
            "display": display,
            "pastedContents": pasted,
            "timestamp": epoch_ms(clock),
            "project": project,
            "sessionId": session_id,
        }

    paste = {"1": {"contentHash": PASTE_HASH, "id": 1, "type": "text"}}
    return [
        line(
            "08:20:14.903",
            "port the CSV export to the new ledger schema and keep the old column order",
            "/srv/invoicing-old",
            REMOVED_REPOSITORY_SESSION,
        ),
        line(
            "08:41:37.226",
            "why does the reconciler skip invoices dated on the last day of the month?",
            PROJECT,
            DELETED_SESSION,
        ),
        line(
            "09:12:03.087",
            "[Pasted text #1 +%d lines] " % CI_FAILURE.count("\n") + FIRST_PROMPT,
            PROJECT,
            TERMINAL_SESSION,
            **paste,
        ),
        line("09:25:02.904", COMMIT_PROMPT, PROJECT, TERMINAL_SESSION),
        line("09:31:08.772", "/remote-control", PROJECT, TERMINAL_SESSION),
        line("16:56:02.377", LOCAL_PROMPT, PROJECT, TELEPORT_SESSION),
    ]


CODEX_PROMPT = (
    "TestExportCSV_Amounts fails because every amount ends with a space. Find where the CSV "
    "writer adds it and fix it."
)
CODEX_THREAD = make_uuid7("bob codex thread", "14:02:11.204")
CODEX_ROLLOUT = ".codex/sessions/2026/10/27/rollout-2026-10-27T14-02-11-%s.jsonl" % CODEX_THREAD
EXPORT_FAILURE = """\
--- FAIL: TestExportCSV_Amounts (0.00s)
    csv_test.go:42: row 2 amount = "12.50 ", want "12.50"
FAIL
FAIL\tgithub.com/acme--ledger/ledger/internal/export\t0.009s
FAIL"""
EXPORT_GREP = '57:\t\tfmt.Fprintf(w, "%s,%.2f ,%s\\n", row.Date, row.Amount, row.Currency)'
EXPORT_PASS = "ok  \tgithub.com/acme--ledger/ledger/internal/export\t0.008s"
EXPORT_PATCH = """\
*** Begin Patch
*** Update File: internal/export/csv.go
@@ func writeRows(w io.Writer, rows []Row) error {
-\t\tfmt.Fprintf(w, "%s,%.2f ,%s\\n", row.Date, row.Amount, row.Currency)
+\t\tfmt.Fprintf(w, "%s,%.2f,%s\\n", row.Date, row.Amount, row.Currency)
*** End Patch"""
CODEX_ANSWER = (
    "The CSV writer's format string had a space after `%.2f`, so every amount ended with one. "
    "I removed it in internal/export/csv.go, committed that on main, and the export tests pass."
)


def codex_rollout_records() -> list[dict]:
    """Return bob's Codex CLI rollout: the CSV export fix, in the paginated rollout shape."""
    turn = make_uuid7("bob codex turn 1", "14:02:19.880")
    lines: list[dict] = []

    def add(clock: str, record_type: str, payload: dict, **keys) -> None:
        line = {"timestamp": stamp(clock), "ordinal": len(lines), "type": record_type}
        line.update(payload=payload, **keys)
        lines.append(line)

    def message(role: str, name: str, content: list, **keys) -> dict:
        return dict(type="message", id=make_uuid(name), role=role, content=content, **keys)

    def kinds(kind: str) -> dict:
        return {"turn_id": turn, "content_item_kinds": [kind]}

    def completed(item: dict, started: str, finished: str) -> dict:
        return {
            "type": "item_completed",
            "thread_id": CODEX_THREAD,
            "turn_id": turn,
            "item": item,
            "started_at_ms": epoch_ms(started),
            "completed_at_ms": epoch_ms(finished),
        }

    def shell(clock: str, done: str, call: str, command: list[str], output: str, code: int):
        call_id = "call_" + make_base62(call, 24)
        tool = {"type": "function_call", "id": make_uuid(call), "name": "exec_command"}
        arguments = json.dumps({"cmd": shlex.join(command)}, separators=(",", ":"))
        tool.update(arguments=arguments, call_id=call_id)
        add(clock, "response_item", tool)
        result = {"type": "function_call_output", "name": "exec_command"}
        add(done, "response_item", dict(result, call_id=call_id, output=output))
        item = {"type": "CommandExecution", "id": call_id, "command": command, "cwd": PROJECT}
        item.update(parsed_cmd=[], source="agent", status="failed" if code else "completed")
        item.update(aggregated_output=output, exit_code=code)
        add(done, "event_msg", completed(item, clock, done))

    meta = {
        "session_id": CODEX_THREAD,
        "id": CODEX_THREAD,
        "timestamp": stamp("14:02:11.204"),
        "cwd": PROJECT,
        "originator": "codex-tui",
        "cli_version": CODEX_VERSION,
        "source": "cli",
        "thread_source": "user",
        "model_provider": "openai",
        "history_mode": "paginated",
        "git": {"commit_hash": MAIN_COMMIT, "branch": "main", "repository_url": ORIGIN_URL},
        "creator_account_id": make_uuid("bob chatgpt account"),
        "creator_user_id": "user-" + make_base62("bob chatgpt user", 24),
    }
    add("14:02:11.204", "session_meta", meta)
    context: dict = {"turn_id": turn, "cwd": PROJECT, "model": CODEX_MODEL}
    context.update(approval_policy="on-request", sandbox_policy={"type": "workspace-write"})
    started = {"type": "task_started", "turn_id": turn, "started_at": int(epoch("14:02:19.880"))}
    started.update(model_context_window=258400, collaboration_mode_kind="default")
    add("14:02:19.880", "event_msg", started)
    add("14:02:19.881", "turn_context", context)
    developer = (
        "<permissions instructions>\nApprovals are on request. The sandbox lets you write "
        "inside the workspace only; ask before anything else.\n</permissions instructions>"
    )
    add(
        "14:02:19.902",
        "response_item",
        message("developer", "codex developer", [{"type": "input_text", "text": developer}]),
    )
    environment = (
        "<environment_context>\n  <cwd>/srv/proj</cwd>\n  <approval_policy>on-request"
        "</approval_policy>\n  <sandbox_mode>workspace-write</sandbox_mode>\n  <network_access>"
        "restricted</network_access>\n  <shell>bash</shell>\n</environment_context>"
    )
    environment_block = [{"type": "input_text", "text": environment}]
    passthrough = {
        "internal_chat_message_metadata_passthrough": kinds("environments.environment_context")
    }
    add(
        "14:02:19.903",
        "response_item",
        message("user", "codex environment", environment_block, **passthrough),
    )
    prompt = [{"type": "input_text", "text": CODEX_PROMPT}]
    typed = {"internal_chat_message_metadata_passthrough": kinds("user.text")}
    add(
        "14:02:19.903",
        "response_item",
        message("user", "codex prompt", prompt, **typed),
        metadata={"client_authored": True, "user_input_order": 0},
    )
    user_item: dict = {"type": "UserMessage", "id": make_uuid("codex prompt item")}
    user_item["content"] = [{"type": "text", "text": CODEX_PROMPT, "text_elements": []}]
    add("14:02:19.903", "event_msg", completed(user_item, "14:02:19.903", "14:02:19.903"))
    test_command = ["go", "test", "./internal/export/"]
    shell("14:02:24.315", "14:02:26.019", "codex first test", test_command, EXPORT_FAILURE, 1)
    grep = ["grep", "-n", "%.2f ", "internal/export/csv.go"]
    shell("14:02:31.442", "14:02:31.497", "codex grep", grep, EXPORT_GREP, 0)
    patch_id = "call_" + make_base62("codex patch", 24)
    call = {"type": "custom_tool_call", "id": make_uuid("codex patch"), "status": "completed"}
    call.update(call_id=patch_id, name="apply_patch", input=EXPORT_PATCH)
    add("14:02:38.760", "response_item", call)
    output = "Success. Updated the following files:\nM internal/export/csv.go"
    result = {"type": "custom_tool_call_output", "name": "apply_patch"}
    text = [{"type": "input_text", "text": output}]
    add("14:02:38.812", "response_item", dict(result, call_id=patch_id, output=text))
    change: dict = {"type": "FileChange", "id": patch_id, "status": "completed"}
    change.update(changes={PROJECT + "/internal/export/csv.go": {"type": "update"}})
    change.update(stdout=output, stderr="")
    add("14:02:38.812", "event_msg", completed(change, "14:02:38.760", "14:02:38.812"))
    shell("14:02:41.106", "14:02:42.731", "codex second test", test_command, EXPORT_PASS, 0)
    commit = ["git", "commit", "-am", "Drop the trailing space from CSV amounts"]
    committed = "[main %s] Drop the trailing space from CSV amounts\n" % BOB_COMMIT[:7]
    committed += " 1 file changed, 1 insertion(+), 1 deletion(-)"
    shell("14:02:44.512", "14:02:44.690", "codex commit", commit, committed, 0)
    answer = [{"type": "output_text", "text": CODEX_ANSWER}]
    add(
        "14:02:47.255",
        "response_item",
        message("assistant", "codex answer", answer, phase="final_answer"),
    )
    complete: dict = {"type": "task_complete", "turn_id": turn, "last_agent_message": CODEX_ANSWER}
    complete.update(started_at=int(epoch("14:02:19.881")), completed_at=int(epoch("14:02:47.301")))
    add("14:02:47.301", "event_msg", dict(complete, duration_ms=27421))
    return lines


def codex_history_records() -> list[dict]:
    """Return bob's `.codex/history.jsonl`: the prompt he typed, `ts` in epoch seconds."""
    return [{"session_id": CODEX_THREAD, "ts": int(epoch("14:02:19")), "text": CODEX_PROMPT}]


CODEX_STATE_SCHEMA = """
create table threads (id text primary key, rollout_path text not null,
  created_at integer not null, updated_at integer not null, source text not null,
  model_provider text not null, cwd text not null, title text not null,
  sandbox_policy text not null, approval_mode text not null,
  tokens_used integer not null default 0, has_user_event integer not null default 0,
  archived integer not null default 0, archived_at integer, git_sha text,
  git_branch text, git_origin_url text, cli_version text not null default '',
  first_user_message text not null default '', created_at_ms integer,
  updated_at_ms integer, model text, thread_source text,
  preview text not null default '', name text);
"""


def write_codex_state(path: Path) -> None:
    """Write bob's `state_5.sqlite` with the `threads` row of his one Codex thread."""
    created, updated = "14:02:11.204", "14:02:47.301"
    connection = sqlite3.connect(str(path))
    try:
        # Codex keeps its stores in write-ahead-log mode; the checkpoint below folds the log
        # back so the main file alone holds the row.
        connection.execute("PRAGMA journal_mode=WAL")
        connection.executescript(CODEX_STATE_SCHEMA)
        connection.execute(
            "insert into threads (id, rollout_path, created_at, updated_at, source,"
            " model_provider, cwd, title, sandbox_policy, approval_mode, tokens_used,"
            " has_user_event, archived, git_sha, git_branch, git_origin_url, cli_version,"
            " first_user_message, created_at_ms, updated_at_ms, model, thread_source, preview)"
            " values (?, ?, ?, ?, 'cli', 'openai', ?, ?, ?, 'on-request', 18342, 1, 0, ?, 'main',"
            " ?, ?, ?, ?, ?, ?, 'user', ?)",
            (
                CODEX_THREAD,
                BOB_HOME + "/" + CODEX_ROLLOUT,
                int(epoch(created)),
                int(epoch(updated)),
                PROJECT,
                CODEX_PROMPT,
                json.dumps({"type": "workspace-write"}),
                MAIN_COMMIT,
                ORIGIN_URL,
                CODEX_VERSION,
                CODEX_PROMPT,
                epoch_ms(created),
                epoch_ms(updated),
                CODEX_MODEL,
                CODEX_PROMPT,
            ),
        )
        connection.commit()
        connection.execute("PRAGMA wal_checkpoint(TRUNCATE)")
    finally:
        connection.close()


# Invented contents for the settings and project files, so their sizes look like a host's.
PROJECT_AGENTS = """\
# Ledger service

A small HTTP service that records invoices for the billing team.

## Build and test

- `make build` compiles the service into `bin/ledger`.
- `make test` runs the unit tests; run it before every commit.
- `make lint` runs the formatter and the linter; fix what it reports.

## Style

- Go code is formatted with `gofmt`; keep functions under 50 lines.
- Every exported function has a doc comment.
- Database migrations go in `migrations/`, one file per change, never edited after merge.
- Do not add a dependency without asking in the pull request.
"""
CURSOR_RULE = """\
---
description: Code style for the ledger service
globs: "**/*.go"
alwaysApply: false
---

- Use tabs for indentation, as gofmt does.
- Return errors with context: `fmt.Errorf("load invoice %d: %w", id, err)`.
- Name tests `TestFunction_Case` and keep them table-driven.
- Never log invoice amounts or customer names.
- Prefer the standard library over a new dependency.
"""
AIDER_HISTORY = "\n# 2026-10-27 08:05:12.481773\n+add tests for the invoice total rounding\n"
AMP_SETTINGS = """\
{
  "amp.notifications.enabled": true,
  "amp.showCosts": false,
  "amp.git.commit.coauthor.enabled": false,
  "amp.updates.mode": "warn",
  "amp.terminal.copyOnSelect": true
}
"""
# No key the gemini-cli parser turns into a row, so the agent stays under `parser found no rows`.
GEMINI_SETTINGS = """\
{
  "general": {
    "vimMode": true,
    "preferredEditor": "vim"
  },
  "ui": {
    "theme": "GitHub",
    "hideBanner": true
  },
  "privacy": {
    "usageStatisticsEnabled": false
  }
}
"""
# bob runs `ollama serve` from his own account, so the host has no `ollama` system user.
OLLAMA_HISTORY = (
    "how do I read a CSV file line by line in Go?\n"
    "what does %.2f do in fmt.Printf, and can it add a trailing space?\n"
    "/bye\n"
)
OLLAMA_MANIFEST = json.dumps(
    {
        "schemaVersion": 2,
        "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
        "config": {
            "mediaType": "application/vnd.docker.container.image.v1+json",
            "digest": "sha256:" + make_hex("qwen config blob", 64),
            "size": 487,
        },
        "layers": [
            {
                "mediaType": "application/vnd.ollama.image.model",
                "digest": "sha256:" + make_hex("qwen model blob", 64),
                "size": 4683071488,
            },
            {
                "mediaType": "application/vnd.ollama.image.template",
                "digest": "sha256:" + make_hex("qwen template blob", 64),
                "size": 1556,
            },
            {
                "mediaType": "application/vnd.ollama.image.license",
                "digest": "sha256:" + make_hex("qwen license blob", 64),
                "size": 11343,
            },
            {
                "mediaType": "application/vnd.ollama.image.params",
                "digest": "sha256:" + make_hex("qwen params blob", 64),
                "size": 59,
            },
        ],
    },
    separators=(",", ":"),
)
OLLAMA_MANIFEST_PATH = ".ollama/models/manifests/registry.ollama.ai/library/qwen2.5-coder/7b"
CODEX_CONFIG = """\
model = "gpt-6.1-sol"
approval_policy = "on-request"
sandbox_mode = "workspace-write"

[projects."/srv/proj"]
trust_level = "trusted"
"""
GIT_HEAD = "ref: refs/heads/main\n"
GIT_EXCLUDE = """\
# git ls-files --others --exclude-from=.git/info/exclude
# Lines that start with '#' are comments.
# For a project mostly in C, the following would be a good set of
# exclude patterns (uncomment them if you want to use them):
# *.[oa]
# *~
"""
GIT_CONFIG = """\
[core]
\trepositoryformatversion = 0
\tfilemode = true
\tbare = false
\tlogallrefupdates = true
[remote "origin"]
\turl = %s
\tfetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
\tremote = origin
\tmerge = refs/heads/main
[remote "upstream"]
\turl = %s
\tfetch = +refs/heads/*:refs/remotes/upstream/*
""" % (ORIGIN_URL, UPSTREAM_URL)


# What the clone at 08:01 and the pull at 13:05 left: the packed clone-time remote ref, and the
# fetched tip of `main` that the pull fast-forwarded to.
PACKED_REFS = (
    "# pack-refs with: peeled fully-peeled sorted \n%s refs/remotes/origin/main\n" % CLONE_COMMIT
)
FETCH_HEAD = "%s\t\tbranch 'main' of github.com:acme--ledger/ledger\n" % MAIN_COMMIT


def gemini_chat() -> dict:
    """Return the chat bob's Gemini CLI wrote, in the one-object form of its older builds."""
    return {
        "sessionId": GEMINI_SESSION,
        "projectHash": hashlib.sha256(PROJECT.encode("utf-8")).hexdigest(),
        "startTime": stamp("09:41:02.118"),
        "lastUpdated": stamp("09:41:55.306"),
        "messages": [],
    }


def _json_text(document: dict) -> str:
    return json.dumps(document, indent=2) + "\n"


def claude_user_settings() -> dict:
    """Return alice's `~/.claude/settings.json`, as `/model` and `/remote-control` left it."""
    permissions = {
        "allow": ["Bash(go test:*)", "Bash(git diff:*)"],
        "deny": ["Bash(git push --force:*)"],
        "defaultMode": "acceptEdits",
    }
    return {"permissions": permissions, "model": "fable", "agentPushNotifEnabled": True}


def claude_global_config() -> dict:
    """Return alice's `~/.claude.json`: the project entry, Remote Control, cloud and GitHub keys."""
    project = {
        "allowedTools": ["Bash(go vet:*)"],
        "hasTrustDialogAccepted": True,
        "remoteControlSpawnMode": "same-dir",
    }
    github = {
        "accountUuid": ALICE_ACCOUNT,
        "orgUuid": ACME_ORGANIZATION,
        "status": "connected",
    }
    return {
        "numStartups": 41,
        "hasUsedRemoteControl": True,
        "remoteControlMachineId": make_uuid("wsdev14 remote control machine"),
        "hasRemoteEnvironment": True,
        "githubWebConnectionStatusCache": github,
        "projects": {PROJECT: project, "/srv/invoicing-old": {"hasTrustDialogAccepted": True}},
    }


def claude_registry_record(pid: int, session_id: str, started: str, updated: str, **keys) -> dict:
    """Return `~/.claude/sessions/<pid>.json` of a session whose process is still running.

    `keys` are added after the fixed ones; `name` is the derived `proj-<2 hex>` form.
    """
    record = {
        "pid": pid,
        "sessionId": session_id,
        "cwd": PROJECT,
        "startedAt": epoch_ms(started),
        "procStart": start_tick(started),
        "version": CLAUDE_VERSION,
        "peerProtocol": 1,
        "peerFeatures": ["messaging"],
        "kind": "interactive",
        "entrypoint": "cli",
        "pidDomain": "linux",
        "name": "proj-" + make_hex("%d name" % pid, 2),
        "nameSince": epoch_ms(started),
        "status": "idle",
        "updatedAt": epoch_ms(updated),
        "statusUpdatedAt": epoch_ms(updated),
    }
    record.update(keys)
    return record


def bridge_pointer() -> dict:
    """Return the project's `bridge-pointer.json`, left by the Remote Control server."""
    return {
        "environmentId": "env_" + make_base62("alice remote control environment", 22),
        "pid": 51877,
        "procStart": start_tick("13:30:12"),
        "sessionId": make_hex("alice remote control server session", 32),
        "source": "standalone",
    }


def ccr_tip() -> dict:
    """Return the spawned session's `ccr-tip.json`: the last claude.ai event it processed."""
    return {"eventId": make_uuid("alice spawn last event"), "updatedAt": stamp("13:31:47.918")}


class SkippedDirectory(NamedTuple):
    """A directory an `EXCLUDES` line matched: one `skipped_excluded` row, nothing under `fs/`."""

    user: str
    home: str
    agent: str
    rel: str
    size: int


class FileEntry(NamedTuple):
    """One file of the collection, as a manifest row and its bytes in `fs/` describe it."""

    user: str
    home: str
    agent: str
    rel: str
    data: bytes
    mode: str
    flagged: bool
    mtime: str
    uid: int


def _sqlite_bytes() -> bytes:
    with tempfile.TemporaryDirectory(prefix="doubleagent-example-") as work:
        path = Path(work) / "state_5.sqlite"
        write_codex_state(path)
        return path.read_bytes()


def files() -> Iterator[FileEntry | SkippedDirectory]:
    """Yield every row of the collection in manifest order, home rows before project rows.

    A file's `mtime` is its modification time on the example's day, `HH:MM:SS`; `agent` is
    what the collector records, and `flagged` is true where a `SECRET_GLOBS` line matches. An
    excluded directory comes before the files of the catalog entry that reached it, as the
    collector writes it, with the size `du -sk` reports.
    """
    yield SkippedDirectory("alice", ALICE_HOME, "claude-code", ".claude/cache", 3168 * 1024)
    claude = ".claude/projects/-srv-proj/"
    alice = [
        (claude + TERMINAL_SESSION + ".jsonl", _jsonl(claude_terminal_records()), "11:02:57"),
        (claude + SPAWN_SESSION + ".jsonl", _jsonl(claude_remote_control_records()), "13:46:20"),
        (claude + SPAWN_SESSION + "/ccr-tip.json", _compact(ccr_tip()), "13:31:47"),
        (claude + TELEPORT_SESSION + ".jsonl", teleport_transcript(), "16:56:08"),
        (claude + "bridge-pointer.json", _compact(bridge_pointer()), "13:30:14"),
        (".claude/history.jsonl", _jsonl(claude_history_records()), "16:56:02"),
        (".claude/paste-cache/%s.txt" % PASTE_HASH, CI_FAILURE, "09:12:03"),
        (
            ".claude/sessions/%d.json" % TERMINAL_PID,
            _compact(
                claude_registry_record(
                    TERMINAL_PID,
                    TERMINAL_SESSION,
                    "09:11:58.402",
                    "11:02:57.130",
                    bridgeSessionId=BRIDGE_SESSION,
                )
            ),
            "11:02:57",
        ),
        (
            ".claude/sessions/%d.json" % TELEPORT_PID,
            _compact(
                claude_registry_record(
                    TELEPORT_PID, TELEPORT_SESSION, "16:55:40.871", "16:56:08.940"
                )
            ),
            "16:56:08",
        ),
        _peer_key(TERMINAL_PID, "09:11:58.402"),
        _peer_key(TELEPORT_PID, "16:55:40.871"),
        (".claude/settings.json", _json_text(claude_user_settings()), "09:31:10"),
        (".claude.json", _json_text(claude_global_config()), "16:56:02"),
    ]
    for rel, text, mtime in alice:
        yield _entry("alice", ALICE_HOME, "claude-code", rel, text, mtime)
    yield _entry("alice", ALICE_HOME, "amp", ".config/amp/settings.json", AMP_SETTINGS, "08:03:51")
    # In the catalog's order: Gemini CLI before Codex CLI, Ollama last.
    bob = [
        ("gemini-cli", ".gemini/settings.json", GEMINI_SETTINGS, "08:47:02"),
        (
            "gemini-cli",
            GEMINI_CHAT,
            _json_text(gemini_chat()),
            "09:41:55",
        ),
        ("codex-cli", CODEX_ROLLOUT, _jsonl(codex_rollout_records()), "14:02:47"),
        ("codex-cli", ".codex/history.jsonl", _jsonl(codex_history_records()), "14:02:19"),
        ("codex-cli", ".codex/state_5.sqlite", _sqlite_bytes(), "14:02:47"),
        ("codex-cli", ".codex/config.toml", CODEX_CONFIG, "13:58:40"),
    ]
    for agent, rel, data, mtime in bob:
        yield _entry("bob", BOB_HOME, agent, rel, data, mtime)
    yield SkippedDirectory("bob", BOB_HOME, "ollama", ".ollama/models/blobs", 4573340 * 1024)
    ollama = [
        ("ollama", ".ollama/history", OLLAMA_HISTORY, "13:44:09"),
        ("ollama", OLLAMA_MANIFEST_PATH, OLLAMA_MANIFEST, "08:46:12"),
    ]
    for agent, rel, data, mtime in ollama:
        yield _entry("bob", BOB_HOME, agent, rel, data, mtime)
    # In the project catalog's order: `.claude`, the shared files, `.cursor`, then Aider's.
    project = [
        ("claude-code", ".claude/settings.local.json", _json_text(_local_settings()), "09:12:53"),
        ("project", "AGENTS.md", PROJECT_AGENTS, "08:01:12"),
        ("project", ".git/HEAD", GIT_HEAD, "13:05:21"),
        ("project", ".git/config", GIT_CONFIG, "08:03:40"),
        ("project", ".git/FETCH_HEAD", FETCH_HEAD, "13:05:27"),
        ("project", ".git/packed-refs", PACKED_REFS, "08:01:12"),
        ("project", ".git/refs/heads/" + FIX_BRANCH, FIX_COMMIT_FULL + "\n", "09:25:15"),
        ("project", ".git/refs/heads/main", BOB_COMMIT + "\n", "14:02:44", "bob"),
        (
            "project",
            ".git/refs/remotes/origin/HEAD",
            "ref: refs/remotes/origin/main\n",
            "08:01:12",
        ),
        ("project", ".git/refs/remotes/origin/main", MAIN_COMMIT + "\n", "13:05:27"),
        ("project", ".git/info/exclude", GIT_EXCLUDE, "08:01:12"),
        ("cursor", ".cursor/rules/style.mdc", CURSOR_RULE, "08:01:12"),
        ("aider", ".aider.input.history", AIDER_HISTORY, "08:05:12"),
    ]
    for agent, rel, data, mtime, *owner in project:
        yield _entry("alice", PROJECT, agent, rel, data, mtime, *owner)


def _peer_key(pid: int, started: str) -> tuple[str, str, str]:
    """Return the `sessions/<pid>.<sha256>.key` file of a running process, a credential."""
    rel = ".claude/sessions/%d.%s.key" % (pid, make_hex("%d key name" % pid, 64))
    token = {"peerToken": make_hex("%d peer token" % pid, 32), "pidDomain": "linux"}
    return rel, _compact(dict(token, procStart=start_tick(started))), started[:8]


def _local_settings() -> dict:
    # The allow-always answer to `make test` writes one wildcard prefix rule and nothing else here.
    return {"permissions": {"allow": ["Bash(make *)"]}}


# Files a `SECRET_GLOBS` line matches, beside every `sessions/*.key` file.
FLAGGED_FILES = {".claude.json", ".codex/config.toml"}
# Directories whose transcripts, cached pastes and registry files Claude Code writes with mode 0600.
PRIVATE_PREFIXES = (".claude/projects/", ".claude/paste-cache/", ".claude/sessions/")


def _mode(rel: str) -> str:
    """Return the file's mode: 0600 where the agent sets it, else the users' umask of 002."""
    if rel in (".claude.json", ".ollama/history") or rel.endswith(".key"):
        return "600"
    sidecar = rel.endswith(("bridge-pointer.json", "ccr-tip.json"))
    if rel.startswith(PRIVATE_PREFIXES) and not sidecar:
        return "600"
    if rel == ".git/info/exclude":
        # Copied from git's template directory, which keeps the template's own mode.
        return "644"
    return "664"


def _compact(document: dict) -> str:
    return json.dumps(document, separators=(",", ":"))


def _entry(user, home, agent, rel, data, mtime, owner=None) -> FileEntry:
    raw = data if isinstance(data, bytes) else data.encode("utf-8")
    flagged = rel in FLAGGED_FILES or rel.endswith(".key")
    uid = UIDS[owner or user]
    return FileEntry(user, home, agent, rel, raw, _mode(rel), flagged, mtime, uid)


# Files the manifest lists as collected but `fs/` lacks, as if lost after the collection.
MISSING = {BOB_HOME + "/" + GEMINI_CHAT}
# The file whose manifest row is cut short, so the examples show the `note:` line a row that
# does not parse gives; the file itself and the summary's counts are those of the whole row.
CUT_PATH = BOB_HOME + "/" + OLLAMA_MANIFEST_PATH
# The accounts `getent passwd` lists on this host whose home exists, in passwd order: the
# collector scans the system accounts' homes too and finds nothing in them.
SCANNED_HOMES = [
    ("root", "/root"),
    ("games", "/usr/games"),
    ("man", "/var/cache/man"),
    ("mail", "/var/mail"),
    ("backup", "/var/backups"),
    ("alice", ALICE_HOME),
    ("bob", BOB_HOME),
]
STARTED = "16:56:10"
FINISHED = "16:56:28"


def manifest_row(entry: FileEntry | SkippedDirectory) -> dict:
    """Return the manifest row of `entry`, every field in the documented order."""
    path = entry.home + "/" + entry.rel
    if isinstance(entry, SkippedDirectory):
        # The sh collector sizes an excluded directory with du and records no times or owner.
        row: dict = {"user": entry.user, "home": entry.home, "agent": entry.agent, "path": path}
        row.update(archive_path="", type="dir", size=entry.size)
        row.update(mtime=0, atime=0, ctime=0, btime=0, uid=0, gid=0, mode="0", sha256="")
        row.update(secret=False, status="skipped_excluded", target="", error="")
        return row
    mtime = int(epoch(entry.mtime))
    uid = entry.uid
    return {
        "user": entry.user,
        "home": entry.home,
        "agent": entry.agent,
        "path": path,
        "archive_path": "fs" + path,
        "type": "file",
        "size": len(entry.data),
        "mtime": mtime,
        "atime": mtime,
        "ctime": mtime,
        "btime": mtime,
        "uid": uid,
        "gid": PROJECT_GID if entry.home == PROJECT else uid,
        "mode": entry.mode,
        "sha256": hashlib.sha256(entry.data).hexdigest(),
        "secret": entry.flagged,
        "status": "collected",
        "target": "",
        "error": "",
    }


def summary(rows: list[dict]) -> dict:
    """Return `collection.json` for the manifest `rows`, in the sh collector's key order."""
    statuses = (
        "collected",
        "symlink",
        "skipped_excluded",
        "skipped_size",
        "skipped_secret",
        "skipped_special",
        "error_copy",
        "skipped_unmatched_volume",
        "error_read",
    )
    counts: dict = {status: sum(row["status"] == status for row in rows) for status in statuses}
    counts["collected_bytes"] = sum(row["size"] for row in rows if row["status"] == "collected")
    options = {"full": False, "no_secrets": False, "max_file_size_bytes": 268435456}
    options.update(users_filter="", no_docker=False, no_projects=False, projects=[])
    capabilities = {"hash_tool": "sha256sum", "stat_mode": "gnu", "worker_shell": "dash"}
    capabilities["archiver"] = "tar -z"
    return {
        "tool": "collect-agent-artifacts",
        "version": COLLECTOR_VERSION,
        "hostname": HOST,
        "mode": "live",
        "root": "/",
        "platform": "Linux 6.8.0-45-generic x86_64",
        "run_as_uid": "0",
        "started": "2026-10-27T%sZ" % STARTED,
        "finished": "2026-10-27T%sZ" % FINISHED,
        "options": options,
        "capabilities": capabilities,
        "users": [name for name, _ in SCANNED_HOMES],
        "homes": [home for _, home in SCANNED_HOMES],
        "users_with_artifacts": ["alice", "bob"],
        "projects": [PROJECT],
        "counts": counts,
        "docker": {
            "volumes_found": 0,
            "volumes_collected": 0,
            "unreadable": 0,
            "docker_desktop": False,
        },
        "notes": [],
        "archive": "%s_20261027T%sZ_agent-artifacts.tar.gz" % (HOST, STARTED.replace(":", "")),
    }


def collector_log() -> str:
    """Return `collector.log`: the lines the sh collector logs, each after its UTC second."""
    steps = [
        (
            "16:56:10",
            "%s %s starting on %s (Linux 6.8.0-45-generic) mode=live root=/ uid=0"
            % ("collect-agent-artifacts", COLLECTOR_VERSION, HOST),
        ),
        (
            "16:56:10",
            "hash=sha256sum stat=gnu max_file_size=256MB full=0 no_secrets=0 no_docker=0"
            " worker_sh=dash",
        ),
        ("16:56:10", "User alice (%s)" % ALICE_HOME),
        ("16:56:10", "  [claude-code] %s/.claude" % ALICE_HOME),
        ("16:56:11", "  [claude-code] %s/.claude.json" % ALICE_HOME),
        ("16:56:11", "  [amp] %s/.config/amp" % ALICE_HOME),
        ("16:56:11", "User bob (%s)" % BOB_HOME),
        ("16:56:11", "  [gemini-cli] %s/.gemini" % BOB_HOME),
        ("16:56:12", "  [codex-cli] %s/.codex" % BOB_HOME),
        ("16:56:12", "  [ollama] %s/.ollama" % BOB_HOME),
        ("16:56:26", "Project [alice] %s" % PROJECT),
        ("16:56:26", "  [claude-code] %s/.claude" % PROJECT),
        ("16:56:26", "  [project] %s/AGENTS.md" % PROJECT),
        ("16:56:26", "  [project] %s/.git/HEAD" % PROJECT),
        ("16:56:26", "  [project] %s/.git/config" % PROJECT),
        ("16:56:26", "  [project] %s/.git/FETCH_HEAD" % PROJECT),
        ("16:56:26", "  [project] %s/.git/packed-refs" % PROJECT),
        ("16:56:26", "  [project] %s/.git/refs" % PROJECT),
        ("16:56:27", "  [project] %s/.git/info/exclude" % PROJECT),
        ("16:56:27", "  [cursor] %s/.cursor" % PROJECT),
        ("16:56:27", "  [aider] %s/.aider.input.history" % PROJECT),
        ("16:56:30", "Archiving"),
    ]
    return "".join("%s %s\n" % (stamp(clock + ".000")[:-5] + "Z", text) for clock, text in steps)


def build_collection(root: Path) -> Path:
    """Write the `wsdev14` collection under `root` and return its directory.

    Args:
        root: An empty directory; the collection is written to `root/wsdev14`.

    Returns:
        The collection directory: `collection.json`, `manifest.jsonl`, `collector.log` and `fs/`.
    """
    collection = root / HOST
    (collection / "fs").mkdir(parents=True)
    rows: list[dict] = []
    lines: list[str] = []
    for entry in files():
        row = manifest_row(entry)
        rows.append(row)
        line = json.dumps(row, separators=(",", ":"))
        if row["path"] == CUT_PATH:
            line = line[: line.index(CUT_PATH) + len(BOB_HOME) + 20]
        lines.append(line)
        if isinstance(entry, SkippedDirectory) or row["path"] in MISSING:
            continue
        disk = collection / row["archive_path"]
        disk.parent.mkdir(parents=True, exist_ok=True)
        disk.write_bytes(entry.data)
        os.utime(disk, (row["mtime"], row["mtime"]))
    (collection / "manifest.jsonl").write_text("\n".join(lines) + "\n", encoding="utf-8")
    (collection / "collector.log").write_text(collector_log(), encoding="utf-8")
    text = json.dumps(summary(rows), indent=2) + "\n"
    (collection / "collection.json").write_text(text, encoding="utf-8")
    return collection


def main(argv: list[str] | None = None) -> int:
    """Write the collection into the directory named by the one argument; return the exit status."""
    arguments = sys.argv[1:] if argv is None else argv
    if len(arguments) != 1:
        print("usage: example_collection.py <empty directory>", file=sys.stderr)
        return 2
    root = Path(arguments[0])
    if not root.is_dir():
        print("error: %s is not a directory" % root, file=sys.stderr)
        return 2
    if any(root.iterdir()):
        print("error: %s is not empty" % root, file=sys.stderr)
        return 2
    print(build_collection(root))
    return 0


if __name__ == "__main__":
    sys.exit(main())
