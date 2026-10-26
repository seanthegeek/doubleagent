#!/usr/bin/env python3
"""Regenerate the real-run examples in ``analyzer/docs/output.md``."""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
from importlib import import_module
from pathlib import Path, PurePosixPath

ANALYZER = Path(__file__).resolve().parents[1]
REPO = ANALYZER.parent
DOC = ANALYZER / "docs" / "output.md"
COLLECTOR = REPO / "collectors" / "collect-agent-artifacts.sh"
FIXTURE_EVENT_MTIME = 1_790_852_400  # Match fixture event time: 2026-10-01T11:00:00Z.
DOC_TEMP_ROOT = "/tmp/doubleagent-output-examples"

sys.path.insert(0, str(ANALYZER))
sys.path.insert(0, str(ANALYZER / "tests"))
FIXTURE_MODULE = import_module("fixtures")
CLAUDE_295_SESSION = FIXTURE_MODULE.CLAUDE_295_SESSION
build_image = FIXTURE_MODULE.build_image


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


def shell_path(shell: str, path: Path, *, cwd: Path, env: dict[str, str]) -> str:
    return run([shell, "-c", 'cd -- "$1" && pwd -P', "sh", str(path)], cwd=cwd, env=env).strip()


def normalize_output_paths(text: str, *, temporary_root: str, image_root: str) -> str:
    text = text.replace(image_root.rstrip("/") + "/", "/")
    return text.replace(temporary_root.rstrip("/"), DOC_TEMP_ROOT)


def normalize_fixture(image: Path) -> None:
    """Make text fixtures and their metadata stable across operating systems."""
    for path in image.rglob("*"):
        if not path.is_file() or path.is_symlink():
            continue
        data = path.read_bytes()
        if b"\0" not in data:
            try:
                content = data.decode("utf-8")
            except UnicodeDecodeError:
                pass
            else:
                content = content.replace("\r\n", "\n").replace("\r", "\n")
                with path.open("w", encoding="utf-8", newline="\n") as stream:
                    stream.write(content)
        os.utime(path, (FIXTURE_EVENT_MTIME, FIXTURE_EVENT_MTIME))


def build_collection(parent: Path) -> tuple[Path, str, str]:
    """Run the collector over the full shared fixture and keep its real output."""
    image = build_image(parent / "image")
    normalize_fixture(image)

    shell = shutil.which("sh")
    if shell is None:
        raise RuntimeError("sh is required to run collectors/collect-agent-artifacts.sh")
    output = parent / "collector-output"
    env = os.environ.copy()
    env.update(LC_ALL="C", TZ="UTC")
    temporary_root = shell_path(shell, parent, cwd=REPO, env=env)
    image_root = shell_path(shell, image, cwd=REPO, env=env)
    run(
        [
            shell,
            str(COLLECTOR),
            "--root",
            str(image),
            "--output",
            str(output),
            "--no-docker",
            "--keep-staging",
            "--quiet",
        ],
        cwd=REPO,
        env=env,
    )

    staging = sorted(output.glob(".stage-*"))
    if len(staging) != 1:
        raise RuntimeError("the collector did not leave exactly one staging collection")
    collection = parent / "ws01"
    staging[0].rename(collection)

    manifest_path = collection / "manifest.jsonl"
    manifest = [
        (json.loads(line), line) for line in manifest_path.read_text(encoding="utf-8").splitlines()
    ]
    manifest.sort(
        key=lambda item: (
            tuple(part.casefold() for part in PurePosixPath(item[0]["path"]).parts),
            item[1],
        )
    )
    with manifest_path.open("w", encoding="utf-8", newline="\n") as stream:
        for _, line in manifest:
            stream.write(line + "\n")

    summary_path = collection / "collection.json"
    summary = json.loads(summary_path.read_text(encoding="utf-8"))
    summary["hostname"] = "ws01"
    with summary_path.open("w", encoding="utf-8", newline="\n") as stream:
        json.dump(summary, stream, indent=2)
        stream.write("\n")
    return collection, temporary_root, image_root


def cli_output(
    args: list[str],
    *,
    cwd: Path,
    env: dict[str, str],
    temporary_root: str,
    image_root: str,
) -> str:
    output = run([sys.executable, "-m", "doubleagent", *args], cwd=cwd, env=env)
    return normalize_output_paths(output, temporary_root=temporary_root, image_root=image_root)


def timeline_example_rows(text: str) -> str:
    """Return every row from the fixture's short Claude Code 2.1.295 session."""
    selected = []
    for line in text.splitlines():
        row = json.loads(line)
        if (
            row["user"] == "alice"
            and row["agent"] == "claude-code"
            and row["session_id"] == CLAUDE_295_SESSION
        ):
            selected.append(line)
    if not selected:
        raise RuntimeError("the fixture timeline has no rows for the selected session")
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
        collection, temporary_root, image_root = build_collection(parent)
        env = os.environ.copy()
        old_pythonpath = env.get("PYTHONPATH", "")
        env["PYTHONPATH"] = str(ANALYZER) + (os.pathsep + old_pythonpath if old_pythonpath else "")

        detected = cli_output(
            ["detect", str(collection.resolve()), "--json"],
            cwd=parent,
            env=env,
            temporary_root=temporary_root,
            image_root=image_root,
        ).rstrip()
        output_dir = parent / "out"
        timeline_stdout = (
            cli_output(
                [
                    "timeline",
                    str(collection.resolve()),
                    "--agent",
                    "claude-code",
                    "-o",
                    "out",
                ],
                cwd=parent,
                env=env,
                temporary_root=temporary_root,
                image_root=image_root,
            )
            .replace("\\", "/")
            .rstrip()
        )
        inventory = (output_dir / "inventory.json").read_text(encoding="utf-8")
        timeline = (output_dir / "timeline.jsonl").read_text(encoding="utf-8")
        sessions = (output_dir / "sessions.jsonl").read_text(encoding="utf-8")
        document = DOC.read_text(encoding="utf-8")
        outputs = {
            "detect-json": markdown_block("json", detected),
            "timeline-stdout": markdown_block("text", timeline_stdout),
            "inventory-json": markdown_block(
                "json",
                normalize_output_paths(
                    inventory, temporary_root=temporary_root, image_root=image_root
                ).rstrip(),
            ),
            "timeline-jsonl": markdown_block(
                "jsonl",
                timeline_example_rows(
                    normalize_output_paths(
                        timeline, temporary_root=temporary_root, image_root=image_root
                    )
                ),
            ),
            "sessions-jsonl": markdown_block(
                "jsonl",
                normalize_output_paths(
                    sessions, temporary_root=temporary_root, image_root=image_root
                ).rstrip(),
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
                "output.md examples are stale; run python3 analyzer/tests/regenerate_output_examples.py"
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
