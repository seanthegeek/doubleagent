"""MkDocs hooks that make the documentation site read the documents as GitHub does.

The site is built from the Markdown documents where they are in the
repository (mkdocs.yml), and the documents are written for GitHub, so:

- docs/index.md is the site's home page, index.html at the root, and a
  README.md keeps its own URL (research/README.html), as foo.md becomes
  foo.html, instead of MkDocs' index.html. The root README.md is the
  GitHub overview and is not a page (mkdocs.yml excludes it). `on_files`
  sets both.
- Every heading gets the id GitHub gives it. The documents link to sections
  by GitHub's anchors and tools/check_doc_links.py checks the links against
  them; Python-Markdown's toc differs for repeated headings (`added_1` where
  GitHub has `added-1`) and drops text inside `<...>` in a code span.
  `on_page_content` sets the ids from the check's own `heading_anchors()`,
  on the headings, their permalinks, the page's table of contents and the
  set of ids MkDocs checks links to anchors against.
- A code span keeps the spaces at its ends that CommonMark keeps
  (` requested=<id>`, `#### `); Python-Markdown strips them, and some
  documents quote exact prefixes and suffixes that way. `on_page_markdown`
  writes such a span as an HTML <code> element, its end spaces as no-break
  spaces so the browser does not collapse them.
- A link to a file that is not a page (CLAUDE.md, LICENSE, a script) or to a
  directory points at that path on GitHub, as it does when the document is
  read there. `on_page_content` rewrites it.

Standard library only; the MkDocs objects are used by attribute.
"""

from __future__ import annotations

import importlib.util
import logging
import os
import posixpath
import re
import urllib.parse
from typing import Any

log = logging.getLogger("mkdocs.hooks.doubleagent")

SOURCE_URL = "https://github.com/seanthegeek/doubleagent"
HOME_PAGE = "docs/index.md"
HEADING = re.compile(r'<h([1-6])([^>]*?)\sid="([^"]*)"([^>]*)>(.*?)</h\1>', re.S)
HREF = re.compile(r'(<a\s[^>]*?href=")([^"#]*)((?:#[^"]*)?")')
SCHEME = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:")
FENCE = re.compile(r"^[ \t]*(`{3,}|~{3,})")
BACKTICKS = re.compile(r"`+")
MARKDOWN_ACTIVE = {char: "&#%d;" % ord(char) for char in "`*_[]\\|!"}
MARKDOWN_ACTIVE.update({"&": "&amp;", "<": "&lt;", ">": "&gt;"})


def _load_link_check() -> Any:
    """Import tools/check_doc_links.py, the oracle for heading anchors."""
    path = os.path.join(os.path.dirname(__file__), "..", "tools", "check_doc_links.py")
    spec = importlib.util.spec_from_file_location("check_doc_links", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


LINK_CHECK = _load_link_check()

# Set by on_files: the destination path of every file the site publishes,
# and the source path of every excluded file by its would-be URL.
_published: set[str] = set()
_excluded_source: dict[str, str] = {}


def on_files(files: Any, config: Any) -> Any:
    """Publish docs/index.md as the home page and each README.md under its own URL."""
    _published.clear()
    _excluded_source.clear()
    for file in files:
        dest = None
        if file.src_uri == HOME_PAGE:
            dest = "index.html"
        elif file.src_uri.endswith("/README.md"):
            dest = file.src_uri[: -len(".md")] + ".html"
        if dest is not None:
            file.dest_uri = dest
            file.url = dest
            file.__dict__.pop("abs_dest_path", None)
        if file.inclusion.is_excluded():
            _excluded_source[file.dest_uri] = file.src_uri
        else:
            _published.add(file.dest_uri)
    return files


def _code_span(body: str) -> str | None:
    """Return a span's HTML when its end spaces matter, else None.

    CommonMark turns line breaks into spaces and strips one space from each
    end only when the content both starts and ends with one.
    """
    body = body.replace("\n", " ")
    if body.startswith(" ") and body.endswith(" ") and body.strip():
        body = body[1:-1]
    core = body.strip(" ")
    if not core or core == body:
        return None
    lead = len(body) - len(body.lstrip(" "))
    trail = len(body) - len(body.rstrip(" "))
    # Python-Markdown still reads Markdown between inline HTML tags, so
    # every character it acts on is an entity, | included for table cells.
    escaped = "".join(MARKDOWN_ACTIVE.get(char, char) for char in core)
    return "<code>%s%s%s</code>" % ("&nbsp;" * lead, escaped, "&nbsp;" * trail)


def _rewrite_spans(text: str) -> str:
    """Pair backtick runs as CommonMark does, within a paragraph, and rewrite spans."""
    out: list[str] = []
    pos = 0
    while (opening := BACKTICKS.search(text, pos)) is not None:
        run = opening.group(0)
        out.append(text[pos : opening.start()])
        stop = text.find("\n\n", opening.end())
        stop = len(text) if stop < 0 else stop
        closing = None
        for match in BACKTICKS.finditer(text, opening.end(), stop):
            if match.group(0) == run:
                closing = match
                break
        if closing is None:
            out.append(run)
            pos = opening.end()
            continue
        body = text[opening.end() : closing.start()]
        out.append(_code_span(body) or run + body + run)
        pos = closing.end()
    out.append(text[pos:])
    return "".join(out)


def on_page_markdown(markdown: str, page: Any, config: Any, files: Any) -> str:
    """Write each code span whose end spaces matter, outside fences, as HTML."""
    out: list[str] = []
    chunk: list[str] = []
    fence = ""
    for line in markdown.split("\n"):
        match = FENCE.match(line)
        if fence:
            out.append(line)
            closing = match and match.group(1)[0] == fence[0] and len(match.group(1)) >= len(fence)
            if closing and not line.strip().lstrip(fence[0]):
                fence = ""
        elif match:
            if chunk:
                out.append(_rewrite_spans("\n".join(chunk)))
            chunk = []
            fence = match.group(1)
            out.append(line)
        else:
            chunk.append(line)
    if chunk:
        out.append(_rewrite_spans("\n".join(chunk)))
    return "\n".join(out)


def _rename_toc(items: Any, ids: dict[str, str]) -> None:
    for item in items:
        if item.id in ids:
            item.id = ids[item.id]
        _rename_toc(item.children, ids)


def _set_heading_ids(html: str, page: Any) -> str:
    wanted = LINK_CHECK.heading_anchors(page.file.content_string)
    found = HEADING.findall(html)
    if len(found) != len(wanted):
        log.warning(
            "%s has %d headings in its source and %d in its HTML; toc ids kept",
            page.file.src_uri,
            len(wanted),
            len(found),
        )
        return html
    renamed = {old: new for (_, _, old, _, _), new in zip(found, wanted, strict=True) if old != new}
    _rename_toc(page.toc.items, renamed)
    # MkDocs validates links to anchors against these after every page is
    # rendered (validation: anchors in mkdocs.yml). The old ids stay, since
    # MkDocs recorded the toc's own permalinks to them as links before this.
    if page.present_anchor_ids is not None:
        page.present_anchor_ids |= set(wanted)
    ids = iter(wanted)

    def heading(match: re.Match[str]) -> str:
        level, before, old, after, inner = match.groups()
        new = next(ids)
        inner = inner.replace('href="#%s"' % old, 'href="#%s"' % new)
        return '<h%s%s id="%s"%s>%s</h%s>' % (level, before, new, after, inner, level)

    return HEADING.sub(heading, html)


def _source_link(match: re.Match[str], page_dir: str, docs_dir: str) -> str:
    start, path, rest = match.groups()
    if not path or path.startswith("/") or SCHEME.match(path):
        return match.group(0)
    target = posixpath.normpath(posixpath.join(page_dir, urllib.parse.unquote(path)))
    if target in _published:
        return match.group(0)
    rel = _excluded_source.get(target, target)
    kind = "tree" if os.path.isdir(os.path.join(docs_dir, rel)) else "blob"
    return "%s%s/%s/main/%s%s" % (start, SOURCE_URL, kind, urllib.parse.quote(rel), rest)


def on_page_content(html: str, page: Any, config: Any, files: Any) -> str:
    """Give the headings GitHub's ids and point links to files that are not pages at GitHub."""
    html = _set_heading_ids(html, page)
    page_dir = posixpath.dirname(page.file.url)
    docs_dir = config["docs_dir"]
    return HREF.sub(lambda match: _source_link(match, page_dir, docs_dir), html)
