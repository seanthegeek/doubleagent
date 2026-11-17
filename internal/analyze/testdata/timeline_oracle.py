r"""Write timeline_expected.json: the analyzer's timeline runs over a small collection.

The analyzer's parser list is replaced by fake parsers that emit the rows of
timeline_spec.json for the files it names, so the expected outputs exercise the
command (deduplication, filters, sessions, inventory.json) without real
transcripts. Run it from the repository root, with a scratch directory for the
collection:

    analyzer/.venv/bin/python -I internal/analyze/testdata/timeline_oracle.py . SCRATCH \
        internal/analyze/testdata/timeline_spec.json \
        > internal/analyze/testdata/timeline_expected.json

Every path under the scratch directory is written as `{ROOT}`.
"""

from __future__ import annotations

import contextlib
import io
import json
import sys
import tempfile
from collections.abc import Iterator
from pathlib import Path
from typing import Any

sys.path.insert(0, sys.argv[1] + "/analyzer")

from doubleagent import parsers as parsers_module
from doubleagent.inputs import Artifact
from doubleagent.model import Row
from doubleagent.parsers.base import Options, Parser

FILES = [
    ("alice", "/home/alice", "codex-cli", ".codex/sessions/a.jsonl", "x\n"),
    ("alice", "/home/alice", "codex-cli", ".codex/history.jsonl", "y\n"),
    ("alice", "/home/alice", "claude-code", ".claude/settings.json", "{}\n"),
    ("bob", "/home/bob", "aider", ".aider.chat.history.md", "# hi\n"),
    ("bob", "/home/bob", "windsurf", ".codeium/windsurf/x.pb", "pb"),
    ("bob", "/home/bob", "copilot", ".config/github-copilot/apps.json", "{}"),
    (
        "alice",
        "/srv/proj",
        "project",
        ".git/config",
        '[remote "origin"]\n\turl = git@example.com:a/p.git\n',
    ),
    ("alice", "/srv/proj", "project", ".git/HEAD", "ref: refs/heads/main\n"),
    ("alice", "/srv/proj", "claude-code", ".claude/settings.local.json", "{}\n"),
    ("alice", "/srv/empty", "project", "AGENTS.md", "# x\n"),
]
PROJECTS = ["/srv/proj", "/srv/empty", "c:\\users\\a\\demo"]
RUNS = [
    ("all", []),
    (
        "filtered",
        ["--since", "2026-10-01T09:00:02Z", "--until", "2026-10-03", "--match", "hello|ls", "-i"],
    ),
    ("agent", ["--agent", "claude-code", "--keep-undated", "--since", "2026-10-02"]),
    ("none", ["--agent", "aider"]),
]
OUTPUT_FILES = ("timeline.jsonl", "sessions.jsonl", "inventory.json")


def fake_parser(spec: dict[str, Any]) -> Parser:
    """Return a parser that emits the spec's rows for the files it names."""

    class FakeParser(Parser):
        agent = spec["agent"]
        name = "fake"
        records_sync = spec.get("sync", False)
        records_relay = spec.get("relay", False)
        records_origin = spec.get("origin", False)
        has_secondary = "secondary" in spec

        def wants(self, artifact: Artifact) -> bool:
            """Return whether the spec names the artifact's file."""
            return artifact.rel in spec["rows"]

        def secondary(self, artifact: Artifact) -> bool:
            """Return whether the artifact is the spec's secondary file."""
            return artifact.rel == spec.get("secondary")

        def secondary_tolerance(self, artifact: Artifact) -> float:
            """Return the spec's tolerance."""
            return spec.get("tolerance", 2.0)

        def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
            """Yield the spec's rows of the artifact's file."""
            for fields in spec["rows"].get(artifact.rel, []):
                row = self.base_row(artifact)
                for key, value in fields.items():
                    setattr(row, key, value)
                yield row

    return FakeParser()


def write_collection(root: Path) -> None:
    """Write the extracted collection of FILES under root."""
    (root / "fs").mkdir(parents=True)
    summary = {"hostname": "h1", "projects": PROJECTS}
    (root / "collection.json").write_text(json.dumps(summary), encoding="utf-8")
    manifest = []
    for user, home, agent, rel, text in FILES:
        path = home + "/" + rel
        disk = root / ("fs" + path)
        disk.parent.mkdir(parents=True, exist_ok=True)
        disk.write_text(text, encoding="utf-8")
        manifest.append(
            {
                "user": user,
                "home": home,
                "agent": agent,
                "path": path,
                "archive_path": "fs" + path,
                "type": "file",
                "status": "collected",
                "size": len(text.encode()),
            }
        )
    lines = "".join(json.dumps(row) + "\n" for row in manifest)
    (root / "manifest.jsonl").write_text(lines, encoding="utf-8")


def main() -> None:
    """Run the timeline runs and print their outputs as one JSON object."""
    spec = json.loads(Path(sys.argv[3]).read_text(encoding="utf-8"))
    parsers_module.ALL[:] = [fake_parser(entry) for entry in spec["parsers"]]
    from doubleagent import cli

    work = Path(tempfile.mkdtemp(dir=sys.argv[2]))
    write_collection(work / "col")
    results = {}
    for name, extra in RUNS:
        stdout, stderr = io.StringIO(), io.StringIO()
        target = work / ("out-" + name)
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            status = cli.main(["timeline", str(work / "col"), "-o", str(target), *extra])
        result = {"exit": status, "stdout": stdout.getvalue(), "stderr": stderr.getvalue()}
        for file_name in OUTPUT_FILES:
            result[file_name] = (target / file_name).read_text(encoding="utf-8")
        results[name] = json.loads(json.dumps(result).replace(str(work), "{ROOT}"))
    print(json.dumps(results, indent=1, ensure_ascii=False))


if __name__ == "__main__":
    main()
