r"""Read the remotes and checked-out branch of a collected project's repository.

The collectors gather a project's `.git` metadata files with agent `project`;
this module reads two of them:

* `.git/config`, git's INI-like configuration: a `[remote "<name>"]` section
  (or the deprecated `[remote.<name>]`, whose name git lower-cases) per
  remote, with every `url` and `pushurl` value kept in file order. Section
  and key names match without regard to case. A value may be quoted, holds
  the escapes `\\`, `\"`, `\n`, `\t` and `\b`, ends at a `#` or `;` outside
  quotes, loses only its unprotected surrounding whitespace, and continues
  after a backslash that ends the line outside a comment. A pair may follow
  its header on one line. A line that is neither a header nor a pair, and the
  keys after a malformed header, are skipped; a value git rejects (an
  unterminated quote, an unknown escape) is dropped and reported.
* `.git/HEAD`: `ref: refs/heads/<branch>` while a branch is checked out,
  else the commit hash of a detached HEAD.

Files are read whole from the evidence copy, only when `lstat` says they are
regular files, so a symlink is never followed. A file that cannot be read is
reported by `project_git` as `<path>: <reason>`, never taken for a
repository without remotes.
"""

from __future__ import annotations

import os
import re
import stat
from collections.abc import Iterable
from pathlib import Path
from typing import TypedDict

from .inputs import PROJECT_AGENT, Artifact
from .textlines import ASCII_SPACE, split_lines

CONFIG_REL = ".git/config"
HEAD_REL = ".git/HEAD"

# git allows only space and tab around a section name and before a key's `=`.
_HEADER_PATTERN = re.compile(r'\[[ \t]*([A-Za-z0-9.-]+)(?:[ \t]+"((?:[^"\\]|\\.)*)")?[ \t]*\]')
_KEY_PATTERN = re.compile(r"([A-Za-z][A-Za-z0-9-]*)[ \t]*(?:=(.*))?$")
_ESCAPES = {"\\": "\\", '"': '"', "n": "\n", "t": "\t", "b": "\b"}


class Remote(TypedDict):
    """One remote of a `.git/config`, its values in file order."""

    name: str
    urls: list[str]
    pushurls: list[str]


class ProjectGit(TypedDict):
    """A project repository's remotes and checked-out branch or commit."""

    remotes: list[Remote]
    head: str


def read_text(path: Path) -> tuple[str, str]:
    """Return a whole file as text and `""`, or `""` and why it cannot be read.

    The text is UTF-8 with undecodable bytes replaced. A symlink is never
    followed; it is not a regular file.
    """
    try:
        if not stat.S_ISREG(os.lstat(path).st_mode):
            return "", "not a regular file"
        descriptor = os.open(path, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0))
        with os.fdopen(descriptor, "rb") as handle:
            data = handle.read()
    except OSError as error:
        return "", error.strerror or type(error).__name__
    return data.decode("utf-8", errors="replace"), ""


def _physical_lines(text: str) -> list[str]:
    """Split text at LF or CRLF only, as git does, dropping a leading byte-order mark."""
    if text.startswith("\ufeff"):
        text = text[1:]
    return split_lines(text)


def _parse_value(lines: list[str], index: int, text: str) -> tuple[str, int, str]:
    """Return a config value, the index of its last line, and git's reason to reject it.

    The value starts with `text` on line `index`; quotes and escapes are
    resolved, a trailing comment removed and only unprotected whitespace
    around it trimmed. A backslash ending the line outside a comment
    continues the value; a lone backslash at the end of the file is dropped.
    A rejected value is `""` and an accepted one has the reason `""`.
    """
    # Each character with whether quotes or an escape protect it: only bare
    # whitespace around the value is trimmed, as git trims it, so a quoted
    # path that ends in a space keeps its space.
    out: list[tuple[str, bool]] = []
    quoted = False
    while True:
        position = 0
        continued = False
        while position < len(text):
            character = text[position]
            if character == "\\":
                if position + 1 < len(text):
                    if text[position + 1] not in _ESCAPES:
                        # git rejects the line; the value is not evidence.
                        return "", index, "unsupported escape \\" + text[position + 1]
                    out.append((_ESCAPES[text[position + 1]], True))
                    position += 2
                    continue
                continued = True
                break
            if character == '"':
                quoted = not quoted
            elif character in "#;" and not quoted:
                break
            else:
                out.append((character, quoted))
            position += 1
        if continued and index + 1 < len(lines):
            index += 1
            text = lines[index]
            continue
        # One pass from each end, since the file is untrusted and a value
        # padded with kilobytes of spaces must not cost a pop(0) per space.
        first, last = 0, len(out)
        while first < last and out[first][0] in ASCII_SPACE and not out[first][1]:
            first += 1
        while last > first and out[last - 1][0] in ASCII_SPACE and not out[last - 1][1]:
            last -= 1
        value = "".join(character for character, _ in out[first:last])
        # A quote still open at the end is a value the file ends inside: git
        # rejects the line, and the text so far is not the destination.
        return value, index, "unterminated quoted value" if quoted else ""


def _entries(text: str, errors: list[str]) -> Iterable[tuple[str, str | None]]:
    """Yield the config's entries in order: `("[", header)` or `(key, value)`.

    The key is lower-cased and the value resolved by `_parse_value`. Comment,
    blank and unparseable lines are skipped. A pair git would reject is
    dropped and described in `errors` as `line <n>: <reason>`.
    """
    lines = _physical_lines(text)
    index = 0
    while index < len(lines):
        # Only leading whitespace is dropped here: a value's trailing
        # whitespace is _parse_value's to judge, since `\ ` at the end of a
        # line is an escaped space, not a continuation.
        line = lines[index].lstrip(ASCII_SPACE)
        if line.startswith("["):
            match = _HEADER_PATTERN.match(line)
            if match is None:
                yield "[", line.rstrip(ASCII_SPACE)  # unterminated or malformed
                index += 1
                continue
            yield "[", match.group(0)
            line = line[match.end() :].lstrip(ASCII_SPACE)
        if not line or line[0] in "#;":
            index += 1
            continue
        match = _KEY_PATTERN.match(line)
        if match is None:
            index += 1
            continue
        if match.group(2) is None:
            yield match.group(1).lower(), None
            index += 1
            continue
        start = index + 1
        value, index, error = _parse_value(lines, index, match.group(2))
        if error:
            errors.append("line %d: %s" % (start, error))
        else:
            yield match.group(1).lower(), value
        index += 1


def parse_remotes(text: str, errors: list[str] | None = None) -> list[Remote]:
    """Return the remotes of a `.git/config` text, sorted by name.

    One `{"name", "urls", "pushurls"}` per remote section that sets a
    non-empty `url` or `pushurl`, each list in file order. `pushurls` is what
    the file says, not git's fallback to `urls`. A rejected value is left out
    and, when `errors` is given, described in it.
    """
    remotes: dict[str, Remote] = {}
    current: str | None = None
    for key, value in _entries(text, [] if errors is None else errors):
        if key == "[":
            match = _HEADER_PATTERN.match(value or "")
            current = None
            if match is None:
                continue  # unterminated or malformed: its keys belong to no section
            section, subsection = match.group(1).lower(), match.group(2)
            if subsection is not None:
                if section == "remote":
                    current = re.sub(r"\\(.)", r"\1", subsection)
            elif section.startswith("remote.") and len(section) > len("remote."):
                # git lower-cases the dotted form's subsection, so
                # [remote.Origin] and [remote "origin"] are one remote.
                current = section[len("remote.") :]
            continue
        if current is None or key not in ("url", "pushurl") or not value:
            continue
        remote = remotes.setdefault(current, {"name": current, "urls": [], "pushurls": []})
        (remote["urls"] if key == "url" else remote["pushurls"]).append(value)
    return [remote for _, remote in sorted(remotes.items())]


def parse_head(text: str) -> str:
    """Return the branch a `.git/HEAD` text names, another ref in full, or the commit hash.

    An empty text gives `""`. Only ASCII whitespace is trimmed, since a ref
    may hold other whitespace.
    """
    lines = [line.strip(ASCII_SPACE) for line in _physical_lines(text)]
    first = next((line for line in lines if line), "")
    if first.startswith("ref:"):
        ref = first[len("ref:") :].strip(ASCII_SPACE)
        return ref[len("refs/heads/") :] if ref.startswith("refs/heads/") else ref
    return first


def project_git(
    artifacts: Iterable[Artifact], problems: list[str] | None = None
) -> dict[str, ProjectGit]:
    """Return `{"remotes", "head"}` per project directory among the artifacts.

    Keys are as `cli.project_agents` keys them; a project without the files is
    absent. The exact spelling `.git/config` wins, else the first file whose
    name matches without regard to case, since the PowerShell collector keeps
    the on-disk spelling (`.GIT/CONFIG`). A file that cannot be read, or a pair
    git would reject, is added once to `problems` as `<original path>: <reason>`.
    """
    chosen: dict[str, dict[str, Artifact]] = {}
    for artifact in artifacts:
        if not artifact.home.project or artifact.agent != PROJECT_AGENT:
            continue
        lower = artifact.rel.lower()
        if lower == CONFIG_REL:
            field, exact = "remotes", artifact.rel == CONFIG_REL
        elif lower == HEAD_REL.lower():
            field, exact = "head", artifact.rel == HEAD_REL
        else:
            continue
        path = artifact.home.original or str(artifact.home.disk_path)
        fields = chosen.setdefault(path, {})
        if field not in fields or (exact and fields[field].rel != artifact.rel):
            fields[field] = artifact
    out: dict[str, ProjectGit] = {}
    for path, fields in chosen.items():
        entry: ProjectGit = {"remotes": [], "head": ""}
        out[path] = entry
        for field, artifact in fields.items():
            text, error = read_text(artifact.disk_path)
            errors = [error] if error else []
            if not error:
                if field == "head":
                    entry["head"] = parse_head(text)
                else:
                    entry["remotes"] = parse_remotes(text, errors)
            for message in errors:
                line = "%s: %s" % (artifact.original, message)
                if problems is not None and line not in problems:
                    problems.append(line)
    return out
