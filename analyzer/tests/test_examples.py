"""Tests that the generated examples of docs/output.md match what the analyzer writes now."""

from __future__ import annotations

import contextlib
import importlib.util
import io
import itertools
import json
import re
import stat
import sys
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1] / "tools"


def load_tool(name: str):
    """Import the tool module `tools/<name>.py` and return it."""
    spec = importlib.util.spec_from_file_location(name, TOOLS / (name + ".py"))
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


examples = load_tool("regenerate_examples")
collection = load_tool("example_collection")

RUN = "run python3 analyzer/tools/regenerate_examples.py"


def run_main(*argv: str) -> tuple[int, str, str]:
    """Run the tool's command line in this process; return the exit status, stdout and stderr."""
    stdout, stderr = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        status = examples.main(list(argv))
    return status, stdout.getvalue(), stderr.getvalue()


def test_documented_examples_are_current():
    diffs = examples.check()
    assert diffs == [], "stale examples in docs/output.md; %s:\n%s" % (RUN, "".join(diffs))


def test_generate_is_repeatable():
    assert examples.generate() == examples.generate()


def test_check_reports_a_stale_block(tmp_path):
    document = tmp_path / "output.md"
    text = examples.DOCUMENT.read_text(encoding="utf-8")
    stale = text.replace('"kind": "collected"', '"kind": "loose"', 1)
    document.write_bytes(stale.encode("utf-8"))
    status, stdout, _ = run_main("--check", "--document", str(document))
    assert status == 1
    assert '-  "kind": "loose",' in stdout
    named = [name for name in examples.BLOCKS if "(%s, documented)" % name in stdout]
    assert named == ["detect-json"], named
    assert document.read_text(encoding="utf-8") == stale, "--check must write nothing"
    assert run_main("--document", str(document))[0] == 0
    assert document.read_text(encoding="utf-8") == text


def marker_faults(text: str) -> dict[str, str]:
    """Return the document `text` broken in every way the marker rule rejects, by case name."""
    markers = [
        marker % name for name in examples.BLOCKS for marker in (examples.BEGIN, examples.END)
    ]
    lines = text.split("\n")
    slots = [lines.index(marker) for marker in markers]
    cases = {}
    for first, second in itertools.combinations(range(len(markers)), 2):
        moved = list(lines)
        moved[slots[first]], moved[slots[second]] = markers[second], markers[first]
        cases["swapped %s / %s" % (markers[first], markers[second])] = "\n".join(moved)
    for slot, marker in zip(slots, markers, strict=True):
        cases["missing " + marker] = "\n".join(lines[:slot] + lines[slot + 1 :])
        cases["duplicated " + marker] = "\n".join([*lines[:slot], marker, *lines[slot:]])
    stray = examples.BEGIN % "other"
    cases["stray marker"] = "\n".join([*lines[: slots[1] + 1], stray, *lines[slots[1] + 1 :]])
    return cases


def test_every_marker_fault_fails_and_writes_nothing(tmp_path):
    text = examples.DOCUMENT.read_text(encoding="utf-8")
    cases = marker_faults(text)
    count = len(examples.BLOCKS) * 2
    assert len(cases) == count * (count - 1) // 2 + count + count + 1
    document = tmp_path / "output.md"
    for case, broken in cases.items():
        document.write_bytes(broken.encode("utf-8"))
        for mode in ([], ["--check"]):
            status, stdout, stderr = run_main(*mode, "--document", str(document))
            assert status == 2, (case, mode)
            assert stdout == "" and "error: markers must be" in stderr, (case, mode)
            assert document.read_text(encoding="utf-8") == broken, (case, mode)


def test_generated_blocks_have_their_fence_and_jsonl_lines_parse():
    blocks = examples.generate()
    assert list(blocks) == list(examples.BLOCKS)
    for name, block in blocks.items():
        language = "text" if name in ("detect-text", "timeline-stdout") else "json"
        lines = block.split("\n")
        assert lines[0] == "```" + language and lines[-2:] == ["```", ""], name
        if name.endswith("-jsonl"):
            for line in lines[1:-2]:
                assert isinstance(json.loads(line), dict), (name, line)
    stdout = blocks["timeline-stdout"]
    for label, name in (("rows", "timeline"), ("sessions", "sessions")):
        match = re.search(r"\n%s: +(\d+) in out/%s\.jsonl[ \n]" % (label, name), stdout)
        assert match, label
        lines = blocks[name + "-jsonl"].split("\n")[1:-2]
        assert int(match.group(1)) == len(lines), (label, match.group(1), len(lines))
    assert "\ninventory: out/inventory.json (" in stdout


def jsonl_block(block: str) -> list[dict]:
    """Return the records of a generated `json` block holding one JSON object per line."""
    return [json.loads(line) for line in block.split("\n")[1:-2]]


def test_example_collection_shows_every_documented_row_form():
    blocks = examples.generate()
    rows = jsonl_block(blocks["timeline-jsonl"])
    sessions = jsonl_block(blocks["sessions-jsonl"])
    results = {row["tool_use_id"] for row in rows if row["turn_type"] == "tool_result"}

    def starts(prefix: str, project: str | None = None):
        def test(row: dict) -> bool:
            if project is not None and row["project_path"] != project:
                return False
            return row["text"].startswith(prefix)

        return test

    row_cases = {
        "a tool_use a tool_result joins": lambda row: (
            row["turn_type"] == "tool_use" and row["tool_use_id"] in results
        ),
        "an accepted decision": starts("permission decision: accept"),
        "a rejected decision": starts("permission decision: reject"),
        "a model switch": lambda row: row["text"].startswith("model: ") and " -> " in row["text"],
        "a permission mode": starts("permission mode: "),
        "a project permission setting": starts("permission setting: ", "/srv/proj"),
        "a user permission setting": starts("permission setting: ", ""),
        "a Remote Control origin": starts("session origin: remote (Remote Control"),
        "a teleport origin": starts("session origin: remote (teleport"),
        "a bridge sync": starts("session synced: remote control bridge"),
        "a remote event sync": starts("session synced: remote event"),
        "a cloud session sync": starts("session synced: cloud session"),
        "a Remote Control environment": starts("remote control: environment"),
        "a scratch directory": starts("scratch directory: "),
        "a git remote": starts("git remote: "),
        "a thread record": starts("thread record: "),
        "a source host": starts("source host: github"),
        "a recovered row": lambda row: row["recovered"] == "yes",
        "an unparseable line": starts("parser: unparseable line"),
        "a slash command": starts("slash command: "),
        "an empty timestamp": lambda row: row["timestamp_utc"] == "",
        "a multi-line text": lambda row: "\n" in row["text"],
        "a file edit call": lambda row: row["tool_name"] in ("Edit", "apply_patch"),
    }
    for case, test in row_cases.items():
        assert any(test(row) for row in rows), "no timeline row shows " + case
    session_cases = {
        "a synced session": lambda session: session["synced"] == "yes",
        "a remote session": lambda session: session["origin"] == "remote",
        "a recovered session": lambda session: session["recovered"] == "yes",
        "a session of three tool calls": lambda session: session["tool_calls"] >= 3,
        "a claude-code session": lambda session: session["agent"] == "claude-code",
        "a codex-cli session": lambda session: session["agent"] == "codex-cli",
    }
    for case, test in session_cases.items():
        assert any(test(session) for session in sessions), "no session shows " + case
    assert "\n  remotes: origin " in blocks["detect-text"]
    stdout = blocks["timeline-stdout"]
    assert "\nnote:" in stdout and "\nproblem:" in stdout
    dropped = re.search(r"\ndropped: +(\d+) duplicate rows", stdout)
    assert dropped and int(dropped.group(1)) > 0, stdout


def test_example_collection_builds_on_disk(tmp_path):
    target = tmp_path / "ex"
    target.mkdir()
    with contextlib.redirect_stdout(io.StringIO()) as stdout:
        assert collection.main([str(target)]) == 0
    built = target / "wsdev14"
    assert stdout.getvalue() == "%s\n" % built
    for name in ("collection.json", "manifest.jsonl", "collector.log"):
        assert (built / name).is_file(), name
    assert (built / "fs").is_dir()
    occupied = tmp_path / "occupied"
    occupied.mkdir()
    (occupied / "notes.txt").write_text("keep\n", encoding="utf-8")
    missing = tmp_path / "missing"
    for case, path in {"not empty": occupied, "missing": missing}.items():
        with contextlib.redirect_stderr(io.StringIO()) as stderr:
            assert collection.main([str(path)]) == 2, case
        assert stderr.getvalue().startswith("error: "), case
    assert [path.name for path in occupied.iterdir()] == ["notes.txt"]
    assert (occupied / "notes.txt").read_text(encoding="utf-8") == "keep\n"
    assert not missing.exists()


def test_unreadable_document_exits_2(tmp_path):
    text = examples.DOCUMENT.read_bytes()
    cases = {
        "missing": tmp_path / "absent.md",
        "not UTF-8": tmp_path / "latin1.md",
        "lone CR": tmp_path / "cr.md",
        "CRLF": tmp_path / "crlf.md",
    }
    cases["not UTF-8"].write_bytes(b"caf\xe9\n")
    with_cr = text.replace(b"Its keys, in order:", b"Its keys,\rin order:", 1)
    assert with_cr != text, "the lone-CR case needs the phrase it edits"
    cases["lone CR"].write_bytes(with_cr)
    cases["CRLF"].write_bytes(text.replace(b"\n", b"\r\n"))
    before = {case: path.read_bytes() for case, path in cases.items() if path.exists()}
    for case, document in cases.items():
        for mode in ([], ["--check"]):
            status, stdout, stderr = run_main(*mode, "--document", str(document))
            assert (status, stdout) == (2, ""), (case, mode)
            assert stderr.startswith("error: "), (case, mode)
    for case, data in before.items():
        assert cases[case].read_bytes() == data, case
    assert not cases["missing"].exists()


def test_failed_run_exits_2_and_writes_nothing(tmp_path, monkeypatch):
    document = tmp_path / "output.md"
    document.write_bytes(examples.DOCUMENT.read_bytes())

    def exits_3(argv):
        return 3

    def raises(argv):
        raise ValueError("parser fault")

    for case, fake in {"exit 3": exits_3, "raises": raises}.items():
        monkeypatch.setattr(examples.cli, "main", fake)
        for mode in ([], ["--check"]):
            status, stdout, stderr = run_main(*mode, "--document", str(document))
            assert (status, stdout) == (2, ""), (case, mode)
            assert stderr.startswith("error: "), (case, mode)
            assert document.read_bytes() == examples.DOCUMENT.read_bytes(), (case, mode)


def test_rewrite_keeps_the_file_mode(tmp_path):
    document = tmp_path / "output.md"
    document.write_bytes(examples.DOCUMENT.read_bytes())
    document.chmod(0o640)
    # Windows keeps only the read-only bit, so compare with what the file reports.
    mode = stat.S_IMODE(document.stat().st_mode)
    assert run_main("--document", str(document))[0] == 0
    assert stat.S_IMODE(document.stat().st_mode) == mode
    assert [path.name for path in tmp_path.iterdir()] == ["output.md"], "temporary file left"


def test_symlinked_document_rewrites_its_target(tmp_path):
    target = tmp_path / "output.md"
    text = examples.DOCUMENT.read_text(encoding="utf-8")
    target.write_bytes(text.replace('"kind": "collected"', '"kind": "loose"', 1).encode("utf-8"))
    link = tmp_path / "link.md"
    try:
        link.symlink_to(target)
    except OSError as error:
        raise unittest.SkipTest("cannot create a symlink: %s" % error) from error
    assert run_main("--document", str(link))[0] == 0
    assert link.is_symlink()
    assert target.read_text(encoding="utf-8") == text
