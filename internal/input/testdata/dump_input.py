"""Print what the Python analyzer's open_input returns for one input, for the Go parity test.

    python3 -I dump_input.py <analyzer directory> <input> [--catalog FILE] [--host H] [--work-dir D]

The analyzer directory is put on the import path (`-I` ignores PYTHONPATH). The output is the
form `dumpCollection` in `dump_test.go` writes: one line per kind, host, note, problem, home,
staged path and artifact, fields separated by tabs, each field's bytes printed with every byte
outside printable ASCII, and the backslash, as `\\xHH`. Paths under the extraction directory
start `<extract>`. An input that cannot be opened prints `error` alone.
"""

import argparse
import json
import os
import sys


def raw(text):
    """Return a str's bytes: a path's escaped byte as the byte, a lone surrogate in UTF-8 form."""
    out = bytearray()
    for character in text:
        code = ord(character)
        if 0xDC80 <= code <= 0xDCFF:
            out.append(code - 0xDC00)
        else:
            out += character.encode("utf-8", "surrogatepass")
    return bytes(out)


def field(text):
    """Return a field as the dump prints it."""
    return "".join(
        chr(byte) if 0x20 <= byte <= 0x7E and byte != 0x5C else "\\x%02x" % byte
        for byte in raw(text)
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("analyzer")
    parser.add_argument("input")
    parser.add_argument("--catalog")
    parser.add_argument("--host", default="")
    parser.add_argument("--work-dir")
    args = parser.parse_args()
    sys.path[:0] = [args.analyzer, os.path.join(args.analyzer, "tests")]
    from pathlib import Path

    from doubleagent import catalog, inputs

    tables = catalog.parse(Path(args.catalog).read_text("utf-8")) if args.catalog else catalog.load()
    work_dir = Path(args.work_dir) if args.work_dir else None
    try:
        collection = inputs.open_input(Path(args.input), tables, work_dir=work_dir, host=args.host)
    except (OSError, ValueError, inputs.InputError):
        print("error")
        return
    extracted = collection._extracted_dir

    def path(value):
        text = str(value)
        if extracted and (text == extracted or text.startswith(extracted + os.sep)):
            text = "<extract>" + text[len(extracted) :]
        return field(text)

    try:
        lines = ["kind\t" + collection.kind, "host\t" + field(collection.host)]
        lines += ["note\t" + field(note) for note in collection.notes]
        lines += ["problem\t" + field(problem) for problem in collection.problems]
        homes = {id(home): index for index, home in enumerate(collection.homes)}
        for index, home in enumerate(collection.homes):
            lines.append(
                "\t".join(
                    [
                        "home",
                        str(index),
                        path(home.disk_path),
                        field(home.original),
                        field(home.user),
                        field(home.host),
                        str(home.inferred).lower(),
                        str(home.project).lower(),
                    ]
                )
            )
            for rel in sorted(home.staged_paths, key=raw):
                staged = home.staged_paths[rel]
                lines.append("\t".join(["staged", str(index), field(rel), path(staged)]))
        for artifact in collection.artifacts:
            row = artifact.manifest
            manifest = "-" if row is None else field(
                json.dumps(row, ensure_ascii=False, separators=(",", ":"))
            )
            lines.append(
                "\t".join(
                    [
                        "artifact",
                        str(homes[id(artifact.home)]),
                        field(artifact.agent),
                        field(artifact.rel),
                        field(artifact.original),
                        path(artifact.disk_path),
                        manifest,
                    ]
                )
            )
        sys.stdout.write("\n".join(lines) + "\n")
    finally:
        collection.cleanup()


main()
