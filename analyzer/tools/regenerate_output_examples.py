#!/usr/bin/env python3
"""Regenerate the real-run examples in ``analyzer/docs/output.md``."""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path, PurePosixPath

ANALYZER = Path(__file__).resolve().parents[1]
REPO = ANALYZER.parent
DOC = ANALYZER / "docs" / "output.md"
FIXTURE_MTIME = 1_790_852_400  # 2026-10-01T11:00:00Z

FIXTURE_SPEC = importlib.util.spec_from_file_location(
    "output_example_fixtures", ANALYZER / "tests" / "fixtures.py"
)
if FIXTURE_SPEC is None or FIXTURE_SPEC.loader is None:
    raise RuntimeError("could not load analyzer/tests/fixtures.py")
FIXTURE_MODULE = importlib.util.module_from_spec(FIXTURE_SPEC)
FIXTURE_SPEC.loader.exec_module(FIXTURE_MODULE)
build_output_example_image = FIXTURE_MODULE.build_output_example_image


def run(command: list[str], *, cwd: Path, env: dict[str, str] | None = None) -> str:
    result = subprocess.run(
        command,
        cwd=cwd,
        env=env,
        check=False,
        encoding="utf-8",
        capture_output=True,
    )
    if result.returncode:
        raise RuntimeError(
            "command failed (%d): %s\n%s%s"
            % (result.returncode, " ".join(command), result.stdout, result.stderr)
        )
    return result.stdout


def build_collection(parent: Path) -> Path:
    """Build a minimal extracted collection from the curated fake image."""
    image = build_output_example_image(parent / "image")
    for path in image.rglob("*"):
        if path.is_file() and not path.is_symlink():
            os.utime(path, (FIXTURE_MTIME, FIXTURE_MTIME))

    collection = parent / "ws01"
    fs = collection / "fs"
    fs.mkdir(parents=True)
    files = sorted(path for path in (image / "home/alice").rglob("*") if path.is_file())
    project = image / "srv/proj/.git"
    files.extend((project / "config", project / "HEAD"))
    manifest = []
    for source in sorted(files):
        relative = source.relative_to(image).as_posix()
        source_path = "/" + relative
        if relative.startswith("home/alice/"):
            home = "/home/alice"
            user = "alice"
            agent = "claude-code"
        else:
            home = "/srv/proj"
            user = "alice"
            agent = "project"
        archive_path = "fs" + source_path
        destination = collection / PurePosixPath(archive_path)
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, destination)
        manifest.append(
            {
                "user": user,
                "home": home,
                "agent": agent,
                "path": source_path,
                "archive_path": archive_path,
                "type": "file",
                "size": source.stat().st_size,
                "mtime": FIXTURE_MTIME,
                "status": "collected",
            }
        )

    with (collection / "collection.json").open("w", encoding="utf-8", newline="\n") as stream:
        json.dump({"hostname": "ws01", "projects": ["/srv/proj"]}, stream, indent=2)
        stream.write("\n")
    with (collection / "manifest.jsonl").open("w", encoding="utf-8", newline="\n") as stream:
        for row in manifest:
            stream.write(json.dumps(row, separators=(",", ":")) + "\n")
    return collection


def cli_output(args: list[str], *, cwd: Path, env: dict[str, str]) -> str:
    return run([sys.executable, "-m", "doubleagent", *args], cwd=cwd, env=env)


def timeline_example_rows(path: Path) -> str:
    """Select issue-relevant, unmodified rows from the generated timeline."""
    wanted = (
        "permission decision: reject",
        "scheduled task fired:",
        "session origin: remote",
        "session synced:",
        "prompt trigger: promptSource=system turnOrigin=scheduled",
    )
    selected = []
    for line in path.read_text(encoding="utf-8").splitlines():
        row = json.loads(line)
        if row["turn_type"] == "system" and row["text"].startswith(wanted):
            selected.append(line)
    if not selected:
        raise RuntimeError("the fixture timeline has no rows for the generated example")
    return "\n".join(selected)


def markdown_block(language: str, content: str) -> str:
    return "```%s\n%s\n```" % (language, content.rstrip())


def replace_generated_block(document: str, name: str, block: str) -> str:
    begin = "<!-- BEGIN GENERATED: %s -->" % name
    end = "<!-- END GENERATED: %s -->" % name
    start = document.index(begin) + len(begin)
    finish = document.index(end, start)
    return document[:start] + "\n" + block + "\n" + document[finish:]


def generate() -> str:
    with tempfile.TemporaryDirectory(prefix="doubleagent-output-examples-") as temporary:
        parent = Path(temporary)
        build_collection(parent)
        env = os.environ.copy()
        old_pythonpath = env.get("PYTHONPATH", "")
        env["PYTHONPATH"] = str(ANALYZER) + (os.pathsep + old_pythonpath if old_pythonpath else "")

        detected = cli_output(["detect", "ws01", "--json", "--files"], cwd=parent, env=env).rstrip()
        output_dir = parent / "out"
        timeline_stdout = (
            cli_output(["timeline", "ws01", "-o", "out"], cwd=parent, env=env)
            .replace("\\", "/")
            .rstrip()
        )

        document = DOC.read_text(encoding="utf-8")
        outputs = {
            "detect-json": markdown_block("json", detected),
            "timeline-stdout": markdown_block("text", timeline_stdout),
            "inventory-json": markdown_block(
                "json", (output_dir / "inventory.json").read_text(encoding="utf-8").rstrip()
            ),
            "timeline-jsonl": markdown_block(
                "jsonl", timeline_example_rows(output_dir / "timeline.jsonl")
            ),
            "sessions-jsonl": markdown_block(
                "jsonl", (output_dir / "sessions.jsonl").read_text(encoding="utf-8").rstrip()
            ),
        }
        for name, block in outputs.items():
            document = replace_generated_block(document, name, block)
        return document


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check",
        action="store_true",
        help="fail if output.md does not match the generated examples",
    )
    args = parser.parse_args()
    generated = generate()
    current = DOC.read_text(encoding="utf-8")
    if args.check:
        if generated != current:
            print(
                "output.md examples are stale; run python3 analyzer/tools/regenerate_output_examples.py"
            )
            return 1
        print("output.md examples match the fixture run")
        return 0

    with DOC.open("w", encoding="utf-8", newline="\n") as stream:
        stream.write(generated)
    print("Regenerated examples in analyzer/docs/output.md")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
