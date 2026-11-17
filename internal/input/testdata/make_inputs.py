"""Build the large inputs of the Go parity test with the Python analyzer's own builders.

    python3 -I make_inputs.py <analyzer directory> <empty directory>

Writes, under the directory, the example collection (`analyzer/tools/example_collection.py`) as
a directory, a gzip tar, a bzip2 tar and a zip; the full fixture home (`fixtures.build_home`) as a
loose tree, as its own agent directories and as a gzip tar; and the fixture disk image
(`fixtures.build_image`). Prints one line per input: a name, a tab, the input path.
"""

import os
import subprocess
import sys
import tarfile
import zipfile
from pathlib import Path


def tar_of(source, archive, mode):
    """Write the contents of source as a tar archive, members named relative to it."""
    with tarfile.open(archive, mode) as tar:
        for entry in sorted(os.listdir(source)):
            tar.add(source / entry, arcname=entry)


def zip_of(source, archive):
    """Write the contents of source as a zip archive with directory entries."""
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as zip_file:
        for directory, names, files in os.walk(source):
            names.sort()
            for name in names:
                path = Path(directory) / name
                zip_file.write(path, os.path.relpath(path, source))
            for name in sorted(files):
                path = Path(directory) / name
                zip_file.write(path, os.path.relpath(path, source))


def main():
    """Write the test inputs into the directory named on the command line."""
    analyzer, out = Path(sys.argv[1]), Path(sys.argv[2])
    sys.path[:0] = [str(analyzer), str(analyzer / "tests")]
    import fixtures

    tool = analyzer / "tools" / "example_collection.py"
    (out / "example").mkdir()
    built = subprocess.run(
        [sys.executable, "-I", str(tool), str(out / "example")],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    collection = Path(built)
    tar_of(collection, out / "example.tar.gz", "w:gz")
    tar_of(collection, out / "example.tar.bz2", "w:bz2")
    zip_of(collection, out / "example.zip")
    home = fixtures.build_home(out / "loose" / "home" / "alice")
    tar_of(out / "loose", out / "loose.tar.gz", "w:gz")
    image = fixtures.build_image(out / "image")
    inputs = [
        ("example collection", collection),
        ("example gzip tar", out / "example.tar.gz"),
        ("example bzip2 tar", out / "example.tar.bz2"),
        ("example zip", out / "example.zip"),
        ("fixture home tree", out / "loose" / "home"),
        ("fixture home", home),
        ("fixture home gzip tar", out / "loose.tar.gz"),
        ("fixture image", image),
    ]
    for name in (".claude", ".gemini", ".codex", ".copilot", ".config"):
        if (home / name).is_dir():
            inputs.append(("fixture agent directory " + name, home / name))
    for name, path in inputs:
        print("%s\t%s" % (name, path))


main()
