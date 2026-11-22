#!/usr/bin/env python3
"""Print the shape of an agent's files without printing any value.

A real-install check needs the key names, value types and JSONL line types of the files an
agent wrote, never the values (no prompts, no tokens, no account ids), so the output can be
pasted into a research document or a report. "Reading a real install" in
docs/development.md documents the output and the options.

    python3 -I lab/round/tools/shape.py ~/.config/Agent
    python3 -I lab/round/tools/shape.py --enums --depth 3 settings.json sessions/
    python3 -I lab/round/tools/shape.py --max-size 20000000 --skip logs ~/.agent

Standard library only, and no import from the rest of the repository, so the single file can be
copied to a VM and run there alone.

Per file, in sorted path order: a JSON document prints its key tree with each value's shape (a
type, or a string's class); a `.jsonl` or `.ndjson` file prints line counts and byte totals per
`type`, the keys per type, the short-token values at `ENUM_PATHS`, the key sets of the objects
in `NESTED_KEYS`, the `system` subtypes, the content block types of messages and the count of
bad lines; anything else prints its size and, under 1 KiB, a character class.

Keys and paths are redacted: a UUID becomes `<uuid>`, 32 or more hex digits `<hex>`, an e-mail
address `<email>` and an OpenAI `user-` or `org-` id `<user-id>` or `<org-id>`. A key that is an
absolute path, a `win32:` path (Codex Desktop's spelling of a Windows path) or a URL becomes
`<abs-path>` or `<url>`, as does an absolute path after a `<prefix>:` inside a key, and a key
that is a JSON-encoded array is redacted element by element. Keys equal after redaction print
once with a count. Symlinks are listed and never followed.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import stat
import sys
from collections import Counter

DEFAULT_SKIP: tuple[str, ...] = (
    "Cache",
    "Code Cache",
    "GPUCache",
    "DawnGraphiteCache",
    "DawnWebGPUCache",
    "blob_storage",
    "Crashpad",
    "Session Storage",
    "Local Storage",
    "IndexedDB",
    "WebStorage",
    "Shared Dictionary",
    "node_modules",
    ".git",
)

UUID_RE = re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}")
HEX_RUN_RE = re.compile(r"[0-9a-fA-F]{32,}")
EMAIL_RE = re.compile(r"[\w.+-]+@[\w-]+(?:\.[\w-]+)+")
USER_ID_RE = re.compile(r"(?<![A-Za-z0-9])user-[A-Za-z0-9]{20,}")
ORG_ID_RE = re.compile(r"(?<![A-Za-z0-9])org-[A-Za-z0-9]{20,}")
WIN32_PATH_RE = re.compile(r"win32:(?:/|[A-Za-z]:\\)")
# An absolute path after a `<prefix>:` inside a key (`local:C:\Users\...`);
# `://` is left to URL_RE.
EMBEDDED_PATH_RE = re.compile(r"(?<=:)(?:[A-Za-z]:\\|/(?!/))[^\s\"]*")
ISO_DATE_RE = re.compile(
    r"\d{4}-\d{2}-\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2}(?:[.,]\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?)?"
)
ABS_PATH_RE = re.compile(r"/|[A-Za-z]:\\")
URL_RE = re.compile(r"[A-Za-z][A-Za-z0-9+.-]*://")
ENUM_RE = re.compile(r"[a-z0-9_-]{1,16}")
HEX_RE = re.compile(r"[0-9a-fA-F]+")
BASE64_RE = re.compile(r"[A-Za-z0-9+/_-]+={0,2}")
SECRET_KEY_RE = re.compile(r"token|secret|key|passw|cred|auth|cookie|bearer|salt", re.I)

JSONL_EXT = (".jsonl", ".ndjson")
SMALL = 1024


def redact(text: str) -> str:
    """Replace UUIDs, hex runs, e-mail addresses and OpenAI ids inside a key or path."""
    text = UUID_RE.sub("<uuid>", text)
    text = USER_ID_RE.sub("<user-id>", text)
    text = ORG_ID_RE.sub("<org-id>", text)
    text = HEX_RUN_RE.sub("<hex>", text)
    return EMAIL_RE.sub("<email>", text)


def redact_key(key: str) -> str:
    """Redact a key.

    A whole path or URL becomes its class, a JSON-encoded array is redacted element by element,
    an absolute path after a `<prefix>:` becomes `<abs-path>`, and the rest goes through
    `redact`.
    """
    if ABS_PATH_RE.match(key) or WIN32_PATH_RE.match(key):
        return "<abs-path>"
    if URL_RE.match(key):
        return "<url>"
    if key.startswith("["):
        try:
            items = json.loads(key)
        except (ValueError, RecursionError):
            items = None
        if isinstance(items, list):
            return json.dumps(
                [
                    redact_key(item) if isinstance(item, str) else "<%s>" % type(item).__name__
                    for item in items
                ],
                separators=(",", ":"),
            )
    return redact(EMBEDDED_PATH_RE.sub("<abs-path>", key))


def str_class(value: str, enums: bool) -> str:
    """Return a string's class, `str=<value>` for a short token under `enums`, or its length."""
    if UUID_RE.fullmatch(value):
        return "str<uuid>"
    if len(value) in (32, 64) and re.fullmatch(r"[0-9a-f]+", value):
        return "str<hex%d>" % len(value)
    if ISO_DATE_RE.fullmatch(value):
        return "str<iso-date>"
    if ABS_PATH_RE.match(value):
        return "str<abs-path>"
    if URL_RE.match(value):
        return "str<url>"
    if enums and ENUM_RE.fullmatch(value):
        return "str=%s" % value
    return "str(len %d)" % len(value)


class Shaper:
    """Print the key tree of a JSON document to a depth, with or without enum values."""

    def __init__(self, depth: int, enums: bool) -> None:
        """Keep the nesting `depth` to print and whether short strings print as values."""
        self.depth = depth
        self.enums = enums

    def inline(self, value: object, level: int, guard: bool = False) -> tuple[str, dict | None]:
        """Return the one-line shape of a value and the object to expand below it, if any.

        `guard` is set under a credential-like key and disables `--enums`.
        """
        if isinstance(value, dict):
            if not value:
                return "{}", None
            if level > self.depth:
                return "{...}", None
            return "object", value
        if isinstance(value, list):
            if not value:
                return "[]", None
            inner, node = self.inline(value[0], level, guard)
            return "[%d x %s]" % (len(value), inner), node
        if isinstance(value, bool):
            return "bool", None
        if isinstance(value, (int, float)):
            return "number", None
        if value is None:
            return "null", None
        if isinstance(value, str):
            return str_class(value, self.enums and not guard), None
        return type(value).__name__, None

    def lines(self, node: dict, level: int, indent: str, guard: bool = False) -> list[str]:
        """Return one line per redacted key of `node`, and its children's lines below it."""
        groups: dict[str, list[object]] = {}
        for key, value in node.items():
            groups.setdefault(redact_key(str(key)), []).append(value)
        out: list[str] = []
        for key in sorted(groups):
            values = groups[key]
            label = key if len(values) == 1 else "%s (x %d)" % (key, len(values))
            guarded = guard or SECRET_KEY_RE.search(key) is not None
            shape, child = self.inline(values[0], level + 1, guarded)
            out.append("%s%s: %s" % (indent, label, shape))
            if child is not None:
                out.extend(self.lines(child, level + 1, indent + "  ", guarded))
        return out

    def document(self, doc: object, indent: str) -> list[str]:
        """Return the lines of a whole JSON document."""
        shape, child = self.inline(doc, 1)
        out = ["%s%s" % (indent, shape)]
        if child is not None:
            out.extend(self.lines(child, 1, indent + "  "))
        return out


# Dotted key paths whose values are enum-like (a mode, a kind, a decision, a model id, a
# tool name) and are counted per line type in a JSONL summary. A value is counted only
# when it is a short token of letters, digits, `_`, `-`, `.`, `:` and `/`, so free text
# never prints, and a token that is a UUID, a long hex id, an absolute path or a URL
# is redacted to its class; `message.content[].name` and `attachment.type` are the
# tool names and the attachment kinds of Claude Code's transcripts.
ENUM_PATHS: tuple[str, ...] = (
    "attachment.type",
    "entrypoint",
    "mode",
    "message.model",
    "message.content[].name",
    "origin.kind",
    "permissionDecision.decision",
    "permissionDecision.grantScope",
    "permissionDecision.reasonCode",
    "permissionDecision.reasonType",
    "permissionDecision.source",
    "permissionMode",
    "sessionKind",
    "promptSource",
    "requestedModel",
    "sandboxed",
    "subtype",
    "toolDenialKind",
    "toolUseResult.type",
    "turnOrigin",
    "userType",
)
ENUM_VALUE_RE = re.compile(r"^[A-Za-z0-9_./:-]{1,48}$")
# Top-level objects whose own key set is printed per line type (an `attachment` object
# per `attachment.type`), so a sidecar structure's keys show without its values.
NESTED_KEYS: tuple[str, ...] = (
    "attachment",
    "compactMetadata",
    "message",
    "origin",
    "params",
    "pastedContents",
    "permissionDecision",
    "snapshot",
    "toolUseResult",
)


def enum_at(record: object, path: str) -> str | None:
    """Return the enum-like value at a dotted path of `record`, or None.

    A boolean is `true` or `false`, a `[]` segment joins its elements' values with `|`, and a
    value that is absent or not enum-like is None.
    """
    node: object = record
    parts = path.split(".")
    for i, part in enumerate(parts):
        if part.endswith("[]"):
            if not isinstance(node, dict) or not isinstance(node.get(part[:-2]), list):
                return None
            found = sorted(
                {
                    value
                    for value in (
                        enum_at(item, ".".join(parts[i + 1 :])) for item in node[part[:-2]]
                    )
                    if value is not None
                }
            )
            return "|".join(found) if found else None
        if not isinstance(node, dict) or part not in node:
            return None
        node = node[part]
    if isinstance(node, bool):
        return "true" if node else "false"
    if isinstance(node, str) and ENUM_VALUE_RE.match(node):
        # A UUID, a long hex id, an absolute path or a URL fits the token shape too;
        # it prints as its class, as a key would, never as the value.
        return redact_key(node)
    return None


def jsonl_lines(data: bytes, indent: str) -> tuple[str, list[str]]:
    """Return the summary note and the per-type lines of a JSONL file."""
    total = bad = non_object = 0
    counts: Counter[str] = Counter()
    keys: dict[str, set[str]] = {}
    sub_counts: Counter[str] = Counter()
    sub_keys: dict[str, set[str]] = {}
    blocks: dict[str, set[str]] = {}
    sizes: Counter[str] = Counter()
    enums: dict[str, Counter[str]] = {}
    nested: dict[str, set[str]] = {}
    for raw in data.splitlines():
        if not raw.strip():
            continue
        total += 1
        try:
            record = json.loads(raw)
        except ValueError:
            bad += 1
            continue
        if not isinstance(record, dict):
            non_object += 1
            continue
        top = {redact_key(str(k)) for k in record}
        rtype = record.get("type")
        if isinstance(rtype, str):
            label = "type=%s" % redact_key(rtype)
        elif "type" in record:
            label = "type=<%s>" % type(rtype).__name__
        else:
            label = "keys{%s}" % ",".join(sorted(top))
        counts[label] += 1
        sizes[label] += len(raw)
        keys.setdefault(label, set()).update(top)
        for path in ENUM_PATHS:
            value = enum_at(record, path)
            if value is not None:
                enums.setdefault("%s %s" % (label, path), Counter())[value] += 1
        for name in NESTED_KEYS:
            sub = record.get(name)
            if isinstance(sub, dict):
                kind = enum_at(sub, "type") if name == "attachment" else None
                nlabel = "%s %s%s keys" % (label, name, "[%s]" % kind if kind else "")
                nested.setdefault(nlabel, set()).update(redact_key(str(k)) for k in sub)
                inner = [member for member in sub.values() if isinstance(member, dict)]
                if inner:  # a map of objects (`pastedContents`): the union of the inner keys
                    nested.setdefault(nlabel.replace(" keys", ".* keys"), set()).update(
                        redact_key(str(k)) for member in inner for k in member
                    )
        if rtype == "system":
            sub = record.get("subtype")
            slabel = "subtype=%s" % redact_key(sub) if isinstance(sub, str) else "no subtype"
            sub_counts[slabel] += 1
            sub_keys.setdefault(slabel, set()).update(top)
        if rtype in ("user", "assistant"):
            message = record.get("message")
            content = message.get("content") if isinstance(message, dict) else None
            if isinstance(content, list):
                found = blocks.setdefault(label, set())
                for block in content:
                    btype = block.get("type") if isinstance(block, dict) else None
                    found.add(redact_key(btype) if isinstance(btype, str) else "<no type>")
    out: list[str] = []
    for label in sorted(counts):
        out.append("%s%s: %d (%d bytes)" % (indent, label, counts[label], sizes[label]))
        out.append("%s  keys: %s" % (indent, ", ".join(sorted(keys[label]))))
        for elabel in sorted(enum for enum in enums if enum.startswith(label + " ")):
            values = enums[elabel]
            shown = ", ".join("%s=%d" % (token, count) for token, count in sorted(values.items()))
            out.append("%s  %s: %s" % (indent, elabel[len(label) + 1 :], shown))
        for nlabel in sorted(
            nested_label for nested_label in nested if nested_label.startswith(label + " ")
        ):
            out.append(
                "%s  %s: %s" % (indent, nlabel[len(label) + 1 :], ", ".join(sorted(nested[nlabel])))
            )
        if label in blocks:
            out.append("%s  content blocks: %s" % (indent, ", ".join(sorted(blocks[label]))))
        if label == "type=system":
            for slabel in sorted(sub_counts):
                out.append("%s  %s: %d" % (indent, slabel, sub_counts[slabel]))
                out.append("%s    keys: %s" % (indent, ", ".join(sorted(sub_keys[slabel]))))
    if non_object:
        out.append("%snon-object lines %d" % (indent, non_object))
    out.append("%sbad lines %d" % (indent, bad))
    return "jsonl, %d lines" % total, out


def text_class(data: bytes) -> str:
    """Return `hex`, `base64`, `printable` or `binary` for a small file's content."""
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return "binary"
    if "\x00" in text:
        return "binary"
    body = text.strip()
    if body and HEX_RE.fullmatch(body):
        return "hex"
    if body and BASE64_RE.fullmatch(body):
        return "base64"
    return "printable"


def shape_file(path: str, size: int, args: argparse.Namespace) -> tuple[str, list[str]]:
    """Return the note and the body lines of a regular file."""
    if size > args.max_size:
        return "%d bytes, skipped (large)" % size, []
    try:
        with open(path, "rb") as handle:
            data = handle.read()
    except OSError as exc:
        return "unreadable (%s)" % type(exc).__name__, []
    size = len(data)
    if size == 0:
        return "0 bytes, empty", []
    if path.lower().endswith(JSONL_EXT):
        summary, out = jsonl_lines(data, "  ")
        return "%d bytes, %s" % (size, summary), out
    try:
        doc = json.loads(data)
    except (ValueError, RecursionError):
        doc = None
    else:
        shaper = Shaper(args.depth, args.enums)
        return "%d bytes, json" % size, shaper.document(doc, "  ")
    note = "%d bytes, not JSON" % size
    if size < SMALL:
        note += ", %s" % text_class(data)
    return note, []


def walk(root: str, skip: set[str]) -> list[tuple[str, str]]:
    """Return the sorted (relative path, kind) pairs under `root`.

    The kind is `file`, `symlink`, `skipped`, `other` or `unreadable (<error>)`; symlinks are
    never followed.
    """
    found: list[tuple[str, str]] = []

    def rel(path: str) -> str:
        return os.path.relpath(path, root).replace(os.sep, "/")

    def onerror(exc: OSError) -> None:
        if exc.filename:
            found.append((rel(str(exc.filename)), "unreadable (%s)" % type(exc).__name__))

    for dirpath, dirnames, filenames in os.walk(root, onerror=onerror):
        keep = []
        for name in dirnames:
            full = os.path.join(dirpath, name)
            if os.path.islink(full):
                found.append((rel(full), "symlink"))
            elif name in skip:
                found.append((rel(full) + "/", "skipped"))
            else:
                keep.append(name)
        dirnames[:] = keep
        for name in filenames:
            full = os.path.join(dirpath, name)
            try:
                mode = os.lstat(full).st_mode
            except OSError as exc:
                found.append((rel(full), "unreadable (%s)" % type(exc).__name__))
                continue
            if stat.S_ISLNK(mode):
                found.append((rel(full), "symlink"))
            elif stat.S_ISREG(mode):
                found.append((rel(full), "file"))
            else:
                found.append((rel(full), "other"))
    return sorted(found)


def describe(full: str, kind: str, args: argparse.Namespace) -> tuple[str, list[str]]:
    """Return the note and the body lines of a walked entry of `kind`."""
    if kind == "symlink":
        return "symlink, not followed", []
    if kind == "skipped":
        return "directory, skipped", []
    if kind == "other":
        return "not a regular file", []
    if kind != "file":
        return kind, []
    try:
        size = os.lstat(full).st_size
    except OSError as exc:
        return "unreadable (%s)" % type(exc).__name__, []
    return shape_file(full, size, args)


def emit(label: str, note: str, body: list[str], out) -> None:
    """Write one file's block, headed by its redacted path and note."""
    out.write("== %s (%s)\n" % (redact(label), note))
    for line in body:
        out.write(line + "\n")


def run(args: argparse.Namespace, out) -> None:
    """Write the shape of every PATH argument to `out`."""
    skip: set[str] = set(DEFAULT_SKIP) | set(args.skip or ())
    for arg in args.paths:
        out.write("# %s\n" % redact(arg))
        try:
            mode = os.lstat(arg).st_mode
        except OSError as exc:
            out.write("unreadable (%s)\n" % type(exc).__name__)
            continue
        name = os.path.basename(os.path.normpath(arg))
        if stat.S_ISLNK(mode):
            emit(name, "symlink, not followed", [], out)
        elif stat.S_ISDIR(mode):
            for rel, kind in walk(arg, skip):
                note, body = describe(os.path.join(arg, rel.rstrip("/")), kind, args)
                emit(rel, note, body, out)
        elif stat.S_ISREG(mode):
            note, body = describe(arg, "file", args)
            emit(name, note, body, out)
        else:
            emit(name, "not a regular file", [], out)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    """Parse the command line."""
    parser = argparse.ArgumentParser(
        prog="shape.py",
        description="Print key names, value types and JSONL line types of files, never values.",
    )
    parser.add_argument("paths", nargs="+", metavar="PATH", help="file or directory to describe")
    parser.add_argument(
        "--max-size",
        type=int,
        default=1024 * 1024,
        metavar="BYTES",
        help="files larger than this print their size only (default 1 MiB)",
    )
    parser.add_argument(
        "--skip",
        action="append",
        metavar="NAME",
        help="directory name not to enter, repeatable; added to the Chromium cache "
        "directories, node_modules and .git",
    )
    parser.add_argument(
        "--depth",
        type=int,
        default=6,
        metavar="N",
        help="object nesting depth to print; deeper objects print {...} (default 6)",
    )
    parser.add_argument(
        "--enums",
        action="store_true",
        help="print short lower-case strings ([a-z0-9_-], at most 16) as str=<value>, "
        "except under credential-like keys (token, secret, key, passw, cred, auth, cookie)",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    """Print the shape of the paths given and return 0."""
    run(parse_args(argv), sys.stdout)
    return 0


if __name__ == "__main__":
    sys.exit(main())
