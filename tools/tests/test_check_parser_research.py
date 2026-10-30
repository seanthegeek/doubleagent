"""Tests for tools/check_parser_research.py on synthetic parser packages and documents."""

from __future__ import annotations

import ast
import pathlib
import textwrap

import check_parser_research as checker
import drift_common
from check_parser_research import Literal, Selector

PARSERS = "analyzer/doubleagent/parsers"

BASE = """\
    class Parser:
        agent = ""
        reads_agents = ()
        records_sync = False
        records_relay = False
        records_origin = False
        has_secondary = False

        def wants(self, artifact):
            raise NotImplementedError

        def secondary(self, artifact):
            return False


    def error_rows(parser, artifact, errors):
        return "parser: %s: %s" % errors[0]
    """

REGISTRY = """\
    from .alpha import AlphaParser

    ALL = [AlphaParser()]
    """

ALPHA = '''\
    """Alpha transcripts."""

    import re

    from .base import Parser

    LOG_PATTERN = re.compile(r"^\\.alpha/logs/[^/]+\\.jsonl$")


    class AlphaParser(Parser):
        agent = "alpha"

        def wants(self, artifact):
            return bool(LOG_PATTERN.match(artifact.rel))

        def parse(self, artifact, record):
            return record.get("type")
    '''


def write(path: pathlib.Path, text: str) -> pathlib.Path:
    """Write dedented `text` to `path`, creating its directory, and return the path."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(textwrap.dedent(text), encoding="utf-8")
    return path


def document(
    schema: str = "`type`",
    plan: str = "`^\\.alpha/logs/[^/]+\\.jsonl$`",
    past: str = "The agent records none.",
    sync: str = "The agent records none.",
    bad: str = "None.",
    intro: str = "",
) -> str:
    """Return an `alpha` research document with the given Parsing sections."""
    return "\n\n".join(
        [
            "# alpha",
            "Alpha is a test agent. %s" % intro,
            "## Source and evidence",
            "## Collection",
            "Collected: `.alpha/notes.md`.",
            "## Parsing",
            "### Record schema",
            schema,
            "### Parser plan",
            plan,
            "#### Past-use rows and secondary sources",
            past,
            "#### Sync, relay and origin rows",
            sync,
            "#### Bad-line rows",
            bad,
            "## Confidence",
            "High.",
            "",
        ]
    )


def repository(
    root: pathlib.Path,
    alpha: str = ALPHA,
    text: str | None = None,
    modules: dict[str, str] | None = None,
    registry: str = REGISTRY,
) -> pathlib.Path:
    """Write a parsers package and `research/alpha.md` under `root`, and return `root`."""
    write(root / PARSERS / "__init__.py", registry)
    write(root / PARSERS / "base.py", BASE)
    write(root / PARSERS / "alpha.py", alpha)
    for name, body in (modules or {}).items():
        write(root / PARSERS / name, body)
    write(root / "research/alpha.md", document() if text is None else text)
    write(root / "research/README.md", "# Research\n")
    return root


def findings(root: pathlib.Path, kind: str) -> list[drift_common.Finding]:
    """Return the findings of one kind the checker yields for the repository at `root`."""
    return [finding for finding in checker.run(root) if finding.kind == kind]


def details(root: pathlib.Path, kind: str) -> list[str]:
    """Return the details of the findings of one kind."""
    return [finding.detail for finding in findings(root, kind)]


def alpha_with(body: str, header: str = "") -> str:
    """Return the alpha module with `body` as the body of its `parse` method."""
    indented = textwrap.indent(textwrap.dedent(body), " " * 12)
    return ALPHA.replace('            return record.get("type")\n', indented).replace(
        "    import re\n", "    import re\n" + textwrap.indent(textwrap.dedent(header), "    ")
    )


# --- The baseline -----------------------------------------------------------------------------


def test_clean_repository_has_no_finding(tmp_path):
    assert list(checker.run(repository(tmp_path))) == []
    assert checker.main([], repository(tmp_path)) == 0


def test_stale_baseline_entry_fails(tmp_path, capsys):
    root = repository(tmp_path)
    key = "check_parser_research|key-undocumented|gone"
    write(root / "tools/drift-baseline/alpha.txt", key + "\n")
    assert checker.main([], root) == 1
    assert 'stale: remove "%s" from tools/drift-baseline/alpha.txt' % key in capsys.readouterr().out


def test_update_baseline_removes_a_stale_entry_and_records_a_finding(tmp_path):
    root = repository(tmp_path, text=document(schema="none"))
    write(root / "tools/drift-baseline/alpha.txt", "check_parser_research|key-undocumented|gone\n")
    assert checker.main(["--update-baseline"], root) == 0
    text = (root / "tools/drift-baseline/alpha.txt").read_text(encoding="utf-8")
    assert text == "check_parser_research|key-undocumented|type\n"
    assert checker.main([], root) == 0


# --- Rule 1: row texts ------------------------------------------------------------------------


def test_row_text_undocumented_and_documented(tmp_path):
    alpha = alpha_with('return "session title: %s" % record.get("type")')
    root = repository(tmp_path, alpha)
    found = findings(root, "row-text-undocumented")
    assert [(item.detail, item.agent, item.path, item.line) for item in found] == [
        ("session title:", "alpha", PARSERS + "/alpha.py", 17)
    ]
    text = document(schema="`type`; rows `session title: <title>`")
    assert details(repository(tmp_path, alpha, text), "row-text-undocumented") == []


def test_row_text_in_a_fence_or_as_a_bare_label_is_documented(tmp_path):
    alpha = alpha_with('return f"turn ended: {record.get(\'type\')}", "slash command: "')
    root = repository(tmp_path, alpha)
    assert details(root, "row-text-undocumented") == ["turn ended:", "slash command:"]
    fenced = document(schema="`type`, label `slash command`\n\n```text\nturn ended: 3\n```")
    assert details(repository(tmp_path, alpha, fenced), "row-text-undocumented") == []


def test_row_text_outside_the_parsing_part_is_undocumented(tmp_path):
    alpha = alpha_with('return "session title: %s" % record.get("type")')
    text = document().replace("Collected: `.alpha/notes.md`.", "`session title: x`")
    assert details(repository(tmp_path, alpha, text), "row-text-undocumented") == ["session title:"]


def test_is_row_text_forms():
    accepted = [
        "model: %s",
        "session synced:",
        "permission mode: <old> -> <new>",
        "MCP server: name=%s",
        "remote control/bridge: x",
    ]
    rejected = [
        "parser: unparseable line",
        "https://example.test/a?b=c",
        "select key from table where key = ?",
        "Model %s",
        "key=value",
        "note:x",
        "%s: %s=%s",
    ]
    for text in accepted:
        assert checker.is_row_text(text), text
    for text in rejected:
        assert not checker.is_row_text(text), text


def test_constant_prefix_stops_at_the_first_placeholder():
    cases = {
        "model: %s -> %s": "model:",
        "turn: {} done": "turn:",
        "session synced: remote session <id>": "session synced: remote session",
        "rule: %(name)s": "rule:",
        "width: %-5d": "width:",
        "plain: text": "plain: text",
        "%s: x": "",
    }
    for text, prefix in cases.items():
        assert checker.constant_prefix(text) == prefix, text


# --- Rule 2: parser messages ------------------------------------------------------------------


def test_parser_message_documented_anywhere_in_the_parsing_part(tmp_path):
    alpha = alpha_with('return "parser: %s is not a JSON object" % record.get("type")')
    under_schema = document(schema="`type`; `parser: index is not a JSON object`")
    root = repository(tmp_path / "schema", alpha, under_schema)
    assert details(root, "parser-message-undocumented") == []
    under_bad_lines = document(bad="`parser: index is not a JSON object`")
    root = repository(tmp_path / "bad", alpha, under_bad_lines)
    assert details(root, "parser-message-undocumented") == []
    under_collection = document().replace(
        "Collected: `.alpha/notes.md`.", "`parser: index is not a JSON object`"
    )
    root = repository(tmp_path / "collection", alpha, under_collection)
    assert details(root, "parser-message-undocumented") == ["parser: %s is not a JSON object"]


def test_constant_pieces_and_holds_pieces_forms():
    pieces = checker.constant_pieces("parser: %s is not a JSON object")
    assert pieces == ["parser:", "is not a JSON object"]
    assert checker.constant_pieces("parser: {} ({})") == ["parser:", "(", ")"]
    assert checker.holds_pieces("parser: index is not a JSON object", pieces)
    assert not checker.holds_pieces("is not a JSON object parser:", pieces)
    assert not checker.holds_pieces("parser: index", pieces)


# --- Rule 3: file patterns --------------------------------------------------------------------


def test_pattern_undocumented_and_documented_unescaped(tmp_path):
    root = repository(tmp_path, text=document(plan="Read: `.alpha/logs/x.json`."))
    found = findings(root, "pattern-undocumented")
    assert [(item.detail, item.line) for item in found] == [(r"^\.alpha/logs/[^/]+\.jsonl$", 7)]
    unescaped = document(plan="`^.alpha/logs/[^/]+.jsonl$`")
    assert details(repository(tmp_path, text=unescaped), "pattern-undocumented") == []
    fenced = document(plan="```text\n^\\.alpha/logs/[^/]+\\.jsonl$\n```")
    assert details(repository(tmp_path, text=fenced), "pattern-undocumented") == []


def test_pattern_documented_by_a_path_or_glob_it_matches(tmp_path):
    for span in ("`.alpha/logs/*.jsonl`", "`~/.alpha/logs/x.jsonl`", "`$HOME/.alpha/logs/?.jsonl`"):
        root = repository(tmp_path, text=document(plan="Globs (home-relative): %s." % span))
        assert details(root, "pattern-undocumented") == [], span
    for span in ("`.alpha/logs/*.json`", "`.alpha/logs`", "`.alpha/logs/a/x.jsonl`"):
        root = repository(tmp_path, text=document(plan="Globs (home-relative): %s." % span))
        found = details(root, "pattern-undocumented")
        assert found == [r"^\.alpha/logs/[^/]+\.jsonl$"], span


def test_corpus_documents_pattern_by_span_path():
    spans = [".qwen/trustedFolders.json", "trustedFolders", r"^\.other\.json$"]
    corpus = checker.corpus_of(spans, spans)
    assert corpus.documents_pattern(r"^\.qwen/trustedFolders\.json$")
    assert not corpus.documents_pattern(r"^\.qwen/trusted\.json$")
    assert not corpus.documents_pattern(r"^[a-zA-Z]+$")
    assert not corpus.documents_pattern("(")


def test_selectors_are_what_wants_and_secondary_use(tmp_path):
    alpha = '''\
        """Alpha."""

        import re

        from .base import Parser

        BASE = r"^\\.alpha/"
        LOG_PATTERN = re.compile(BASE + r"logs/[^/]+\\.jsonl$")
        HISTORY_REL = ".alpha/history.jsonl"
        TEMPLATE = r"^%s/x$"
        SPACE_PATTERN = re.compile(r"[ \\t]+")


        class AlphaParser(Parser):
            agent = "alpha"
            config_rel = ".alpha/config.toml"
            dynamic = re.compile(TEMPLATE % "y")

            def wants(self, artifact):
                rel = artifact.rel
                return (
                    bool(LOG_PATTERN.match(rel))
                    or rel == self.config_rel
                    or self._other(rel)
                    or bool(self.dynamic.match(rel))
                )

            def secondary(self, artifact):
                return artifact.rel in (HISTORY_REL, "chat") or artifact.rel.endswith(".bak")

            def _other(self, rel):
                return re.match(r"^\\.alpha/other\\.json$", rel)

            def parse(self, artifact, record):
                return SPACE_PATTERN.sub(" ", record.get("type"))
        '''
    root = repository(tmp_path, alpha)
    parser = drift_common.parser_classes(root / PARSERS)[0]
    index = checker.ModuleIndex(root / "analyzer/doubleagent")
    found = checker.selectors(index, parser, root / PARSERS)
    assert sorted((item.literal.text, item.is_regex) for item in found) == [
        (".alpha/config.toml", False),
        (".alpha/history.jsonl", False),
        (".bak", False),
        (r"^\.alpha/logs/[^/]+\.jsonl$", True),
        (r"^\.alpha/other\.json$", True),
    ]


def test_unescape_pattern_forms():
    cases = {
        r"^\.alpha/x\.json$": "^.alpha/x.json$",
        r"a\-b\/c": "a-b/c",
        r"\d+\s\w": r"\d+\s\w",
        "~/.alpha/x": ".alpha/x",
    }
    for text, unescaped in cases.items():
        assert checker.unescape_pattern(text) == unescaped, text


def test_looks_like_pattern_forms():
    for text in (r"^\.a/b$", "[^/]+x", "(?:a|b)", r"a\.b", "x$"):
        assert checker.looks_like_pattern(text), text
    for text in ("history.jsonl", "~/.claude", "message.model", "a/b/c"):
        assert not checker.looks_like_pattern(text), text


# --- Rule 4: keys -----------------------------------------------------------------------------


def test_key_undocumented_and_documented_as_a_word(tmp_path):
    root = repository(tmp_path, text=document(schema="`message-type`"))
    found = findings(root, "key-undocumented")
    assert [(item.detail, item.line) for item in found] == [("type", 17)]
    dotted = document(schema="`envelope.type`")
    assert details(repository(tmp_path, text=dotted), "key-undocumented") == []
    in_other_stores = document(schema="", plan='`x`\n\n```json\n{"type": 1}\n```')
    assert details(repository(tmp_path, text=in_other_stores), "key-undocumented") == []


def test_key_read_forms(tmp_path):
    alpha = alpha_with(
        """\
        a = record.get("got")
        b = record["subscripted"]
        c = "member" in record
        d = record.pop("popped")
        e = record.get(NAMED)
        f = "\\n" in text
        g = record.get("two words")
        h = "x" in ("x", "y")
        i = row["_rowid"]
        record["stored"] = 1
        return a, b, c, d, e, f, g, h, i
        """,
        header='NAMED = "constant"\n',
    )
    root = repository(tmp_path, alpha, document(schema="none"))
    assert sorted(details(root, "key-undocumented")) == [
        "constant",
        "got",
        "member",
        "popped",
        "subscripted",
    ]


def test_sql_names_forms():
    assert checker.sql_names("select rowid as _rowid, key, value from ItemTable where x = ?") == [
        "key",
        "value",
        "ItemTable",
    ]
    assert checker.sql_names("SELECT idx, length(content) AS size, %s FROM steps JOIN b") == [
        "idx",
        "content",
        "steps",
        "b",
    ]
    assert checker.sql_names("select * from sessions order by created_at") == ["sessions"]
    openclaw = (
        "select d.rowid as _rowid, d.*, c.channel from conversation_deliveries d"
        " join conversations c using (conversation_id) order by d.created_at, d.rowid"
    )
    assert checker.sql_names(openclaw) == ["channel", "conversation_deliveries", "conversations"]
    aliased = 'select "t".thread_id, a.* from worktrees as a left join links t on t.id = a.id'
    assert checker.sql_names(aliased) == ["thread_id", "worktrees", "links"]
    assert checker.sql_names("pragma table_info(steps)") == []
    assert checker.sql_names("read from the file") == []


def test_sql_columns_are_keys(tmp_path):
    alpha = alpha_with('return connection.execute("select name, title from chats")')
    root = repository(tmp_path, alpha, document(schema="`chats`: `name`"))
    assert details(root, "key-undocumented") == ["title"]


def test_is_key_forms():
    for text in ("type", "message.model", "ai-title", "$type", "@id", "_rowid"):
        assert checker.is_key(text), text
    for text in ("\n", "<command-name>", "a b", "2x", "", "-x", "a/b"):
        assert not checker.is_key(text), text


def test_corpus_documents_key_bounds():
    corpus = checker.corpus_of(["origin.kind", "ai-title", "chat.store.index"], [])
    assert corpus.documents_key("kind")
    assert corpus.documents_key("ai-title")
    assert corpus.documents_key("chat.store.index")
    assert not corpus.documents_key("title")
    assert not corpus.documents_key("store.ind")


def test_corpus_documents_prefix_and_bare_label():
    corpus = checker.corpus_of(["model: <id>", "slash command"], ["slash command"])
    assert corpus.documents_prefix("model:")
    assert corpus.documents_prefix("slash command:")
    assert not corpus.documents_prefix("slash command: x")
    assert not corpus.documents_prefix("session title:")


# --- Rule 5: flags ----------------------------------------------------------------------------


SYNCED = """\
    from .base import Parser


    class AlphaParser(Parser):
        agent = "alpha"
        records_sync = True
        has_secondary = True

        def parse(self, artifact, record):
            return 1
    """


def test_flag_undocumented_when_the_subsection_records_none(tmp_path):
    root = repository(tmp_path, SYNCED, document(plan="", schema=""))
    found = findings(root, "flag-undocumented")
    assert [(item.detail, item.path, item.line) for item in found] == [
        ("records_sync", PARSERS + "/alpha.py", 4),
        ("has_secondary", PARSERS + "/alpha.py", 4),
    ]
    text = document(
        plan="",
        schema="",
        sync="`session synced: <id>` rows.",
        past="`history.jsonl` is a secondary source.",
    )
    assert findings(repository(tmp_path, SYNCED, text), "flag-undocumented") == []


def test_flag_unset_when_the_subsection_names_the_row(tmp_path):
    text = document(
        sync="A session gets `session origin: remote`. A `session relayed:` row follows.",
        past="`.alpha/history` is read as a secondary source.",
    )
    found = findings(repository(tmp_path, text=text), "flag-unset")
    assert [(item.detail, item.path) for item in found] == [
        ("records_relay", "research/alpha.md"),
        ("records_origin", "research/alpha.md"),
        ("has_secondary", "research/alpha.md"),
    ]
    assert findings(repository(tmp_path), "flag-unset") == []


def test_named_flags_forms(tmp_path):
    cases = {
        "`h` is a secondary source.": {"has_secondary": 5},
        "`h` and `i` are secondary, read on every run.": {"has_secondary": 5},
        "It is read as a secondary source.": {"has_secondary": 5},
        "There is no secondary source.": {},
        "`h` is not a secondary source.": {},
        "Secondary sources: none.": {},
        "Rows read `session synced: <id>`.\nThen `session origin: remote`.": {
            "records_sync": 5,
            "records_origin": 6,
        },
        "The parser yields no `session synced:` row.": {},
        "So neither is secondary.": {},
        "A `session origin: remote` row would need a reader.": {},
        "Never `session relayed:`. Always `session relayed: x`.": {"records_relay": 5},
    }
    for body, expected in cases.items():
        assert checker.named_flags(section_of(tmp_path, body)) == expected, body


def test_states_none_forms():
    assert checker.states_none("The agent records\nnone.")
    assert checker.states_none("Not stated.")
    assert not checker.states_none("The agent records none of the relay kind")


# --- Rule 6: not read -------------------------------------------------------------------------


def test_not_read_contradicted_by_a_key_and_by_a_pattern(tmp_path):
    text = document(
        plan="`^\\.alpha/logs/[^/]+\\.jsonl$`\n\nNot read: `type`, `logs/<id>.jsonl` and `x.db`."
    )
    found = findings(repository(tmp_path, text=text), "not-read-contradicted")
    assert [(item.detail, item.path, item.line) for item in found] == [
        ("type", "research/alpha.md", 21),
        ("logs/<id>.jsonl", "research/alpha.md", 21),
    ]
    clean = document(plan="`^\\.alpha/logs/[^/]+\\.jsonl$`\n\nNot read: `x.db` and `kind`.")
    assert findings(repository(tmp_path, text=clean), "not-read-contradicted") == []


def section_of(tmp_path: pathlib.Path, body: str) -> drift_common.Section:
    """Return the one section of a document holding `body` under a heading."""
    path = write(tmp_path / "doc.md", "# a\n\n## Parsing\n\n" + body)
    return drift_common.research_sections(path)["Parsing"]


def test_not_read_spans_forms(tmp_path):
    cases = {
        "Not read: `a`, `b` (`c` is read only for rows) and `d`.": ["a", "b", "d"],
        "The `a` and `b` records are not read.": ["a", "b"],
        "`file.json` `key` is not read for rows.": ["key"],
        "It reads `a`, but `b` is not read.": ["b"],
        "The parser does not read `a` or `b`; it reads `c`.": ["a", "b"],
        "Holds `a` and `b` (not read by the parser).": ["b"],
        "Its value `a` was not read. Then `b`.": [],
        "Reads `a`. The `b` key is not read.": ["b"],
        "| `a` | `b` (not read) |\n| --- | --- |": ["b"],
        "- `a` is read.\n- `b` is not read.": ["b"],
    }
    for body, expected in cases.items():
        spans = checker.not_read_spans(section_of(tmp_path, body))
        assert [span.text for span in spans] == expected, body


def test_text_units_split_sentences_items_and_cells():
    text = "One `a. B` here. Two.\n- item one\n- item two\n\n| x | y |\nlast"
    units = [text[start:end].strip() for start, end in checker.text_units(text)]
    assert units == [
        "One `a. B` here.",
        "Two.",
        "- item one",
        "- item two",
        "x",
        "y",
        "last",
    ]


def test_mask_code_spans_keeps_offsets_and_backticks():
    assert checker.mask_code_spans("a `b.c` d ``e`` f") == "a `xxx` d ``x`` f"


def test_regex_components_forms():
    cases = {
        r"^\.claude/projects/[^/]+/.*\.jsonl$": [r"\.claude", "projects", "[^/]+", r".*\.jsonl"],
        r"^(?:a/b|c)/[^/]+\.json$": ["(?:a/b|c)", r"[^/]+\.json"],
        r"^x(?:/y)?\.db$": [r"x(?:/y)?\.db"],
        r"a\/b": [r"a\/b"],
    }
    for pattern, components in cases.items():
        assert checker.regex_components(pattern) == components, pattern


def selector(text: str, is_regex: bool) -> Selector:
    """Return a selector of `text` at an arbitrary place."""
    return Selector(Literal(text, pathlib.Path("alpha.py"), 1), is_regex)


def test_selector_matches_name_forms():
    logs = selector(r"^\.alpha/logs/[^/]+\.jsonl$", True)
    history = selector(r"^\.alpha/history\.jsonl$", True)
    config = selector(".alpha/config.toml", False)
    accepted = [
        (logs, "~/.alpha/logs/<id>.jsonl"),
        (logs, "logs/"),
        (logs, "logs/*.jsonl"),
        (history, "history.jsonl"),
        (config, "config.toml"),
        (config, "~/.alpha/config.toml"),
        (config, ".alpha/"),
    ]
    rejected = [
        (logs, "a.jsonl"),
        (logs, "backups/"),
        (logs, ".beta/logs/a.jsonl"),
        (logs, "model"),
        (history, "history.json"),
        (config, "config.json"),
        (config, "toml/"),
    ]
    for item, name in accepted:
        assert checker.selector_matches_name(item, name), (item.literal.text, name)
    for item, name in rejected:
        assert not checker.selector_matches_name(item, name), (item.literal.text, name)


# --- Rule 7: the module docstring -------------------------------------------------------------


def test_docstring_fact_undocumented_and_documented(tmp_path):
    alpha = ALPHA.replace(
        '"""Alpha transcripts."""',
        '"""Alpha transcripts.\n\n    Fields read: `type`, `ghost`; rows `alpha note: <x>`; `^\\\\.alpha/x$`.\n    """',
    )
    root = repository(tmp_path, alpha)
    found = findings(root, "docstring-fact-undocumented")
    assert [(item.detail, item.line) for item in found] == [
        ("ghost", 3),
        ("alpha note: <x>", 3),
        ("^\\.alpha/x$", 3),
    ]
    text = document(intro="`ghost`, `alpha note: y`, `^.alpha/x$`.")
    assert findings(repository(tmp_path, alpha, text), "docstring-fact-undocumented") == []


def test_docstring_key_value_fact(tmp_path):
    alpha = ALPHA.replace(
        '"""Alpha transcripts."""',
        '"""Alpha transcripts.\n\n    A header line has `type: "session"`.\n    """',
    )
    found = findings(repository(tmp_path / "absent", alpha), "docstring-fact-undocumented")
    assert [item.detail for item in found] == ['type: "session"']
    for name, schema in (("json", '`{"type":"session"}`'), ("spans", "`type` and `session`")):
        root = repository(tmp_path / name, alpha, document(schema=schema))
        assert findings(root, "docstring-fact-undocumented") == [], name
    outside = document(schema="`type`", intro="`session`")
    root = repository(tmp_path / "outside", alpha, outside)
    assert [item.detail for item in findings(root, "docstring-fact-undocumented")] == [
        'type: "session"'
    ]


def test_key_value_forms():
    assert checker.key_value('type: "session"') == ("type", "session")
    assert checker.key_value("role: 'user'") == ("role", "user")
    assert checker.key_value('payload.type: "x.y"') == ("payload.type", "x.y")
    for span in ("type: session", 'type: ""', "model: a -> b", 'a b: "c"', 'type:"x"'):
        assert checker.key_value(span) is None, span


def test_docstring_facts_forms():
    code = frozenset({"artifact", "ClaudeParser", "vscode_state"})
    text = (
        "`type`, `message.model`, `session synced:`, `parser: x`, `^a$`, `history.jsonl`, "
        "`artifact.rel`, `ClaudeParser._parse`, `vscode_state.EDITOR_AGENTS`, `TITLE_FIELDS`, "
        "`a/b/c`, `x y`, `kind: 'k'`"
    )
    found = [span for span, _ in checker.docstring_facts(text, code)]
    assert found == ["type", "message.model", "session synced:", "parser: x", "^a$", "kind: 'k'"]


# --- The effective set ------------------------------------------------------------------------


READER = '''\
    """A reader other parsers import."""


    def read_lines(record):
        return helper(record), record.get("reader_key")


    def helper(record):
        return record.get("helper_key"), "reader row: %s" % LABEL


    def unrelated(record):
        return record.get("unrelated_key")


    LABEL = "x"
    '''


def test_effective_set_of_a_reader_import(tmp_path):
    alpha = alpha_with(
        """\
        from . import other
        return read_lines(record), other.shared(record)
        """,
        header="from .reader import read_lines\n",
    )
    other = 'def shared(record):\n    return record.get("other_key")\n\n\ndef not_used(record):\n    return record.get("other_unused")\n'
    root = repository(
        tmp_path, alpha, document(schema="`type`"), {"reader.py": READER, "other.py": other}
    )
    assert sorted(details(root, "key-undocumented")) == ["helper_key", "other_key", "reader_key"]
    assert details(root, "row-text-undocumented") == ["reader row:"]


def test_effective_set_of_a_subclass_holds_its_base_module_and_not_base_py(tmp_path):
    beta = '''\
        """Beta, Alpha's base."""

        from .base import Parser


        class BetaParser(Parser):
            agent = "beta"

            def parse(self, artifact, record):
                return record.get("beta_key")


        def not_imported(record):
            return record.get("beta_module_key")
        '''
    alpha = '''\
        """Alpha transcripts."""

        from .beta import BetaParser


        class AlphaParser(BetaParser):
            agent = "alpha"
        '''
    root = repository(tmp_path, alpha, document(plan=""), {"beta.py": beta})
    assert sorted(details(root, "key-undocumented")) == ["beta_key", "beta_module_key"]
    parser = drift_common.parser_classes(root / PARSERS)[0]
    index = checker.ModuleIndex(root / "analyzer/doubleagent")
    effective = checker.effective_set(index, parser, root / PARSERS)
    assert sorted(path.name for path in effective.nodes) == ["alpha.py", "beta.py"]
    assert details(root, "parser-message-undocumented") == []


def test_string_literals_skip_docstrings_and_f_string_parts():
    tree = ast.parse(
        '"""Module."""\n\n\ndef f(x):\n    """Doc."""\n    return f"a: {x} b: c", "plain"\n'
    )
    found = checker.string_literals(pathlib.Path("m.py"), [tree], checker.docstring_nodes(tree))
    assert [(item.text, item.line) for item in found] == [("a: {} b: c", 6), ("plain", 6)]


def test_fold_forms(tmp_path):
    module = write(
        tmp_path / "pkg/m.py",
        'BASE = "a/"\nWHOLE = BASE + "b"\nPLAIN = f"c"\nRUNTIME = "%s" % BASE\n',
    )
    index = checker.ModuleIndex(tmp_path / "pkg")
    bindings = index.bindings(module)
    assert index.fold(module, bindings["WHOLE"][1]) == "a/b"
    assert index.fold(module, bindings["PLAIN"][1]) == "c"
    assert index.fold(module, bindings["RUNTIME"][1]) is None
    assert index.fold(module, ast.Name("MISSING")) is None


INHERITING_REGISTRY = """\
    from .alpha import AlphaParser
    from .beta import BetaParser

    ALL = [AlphaParser(), BetaParser()]
    """

BETA_BASE = """\
    from .base import Parser


    class BetaParser(Parser):
        agent = "beta"

        def parse(self, artifact, record):
            return record.get("inherited")
    """

ALPHA_CHILD = """\
    from .beta import BetaParser


    class AlphaParser(BetaParser):
        agent = "alpha"

        def wants(self, artifact):
            return artifact.rel.get("own")
    """


def test_inherited_key_is_documented_by_the_base_agents_document(tmp_path):
    root = repository(
        tmp_path,
        ALPHA_CHILD,
        document(schema="`own`", plan=""),
        {"beta.py": BETA_BASE},
        INHERITING_REGISTRY,
    )
    write(
        root / "research/beta.md", document(schema="`inherited`", plan="").replace("alpha", "beta")
    )
    assert findings(root, "key-undocumented") == []
    write(root / "research/beta.md", document(schema="`other`", plan="").replace("alpha", "beta"))
    found = findings(root, "key-undocumented")
    assert [(item.agent, item.detail, item.path, item.line) for item in found] == [
        ("alpha", "inherited", PARSERS + "/beta.py", 8),
        ("beta", "inherited", PARSERS + "/beta.py", 8),
    ]


def test_own_module_literal_is_checked_against_the_own_document_only(tmp_path):
    root = repository(
        tmp_path,
        ALPHA_CHILD,
        document(schema="`inherited`", plan=""),
        {"beta.py": BETA_BASE},
        INHERITING_REGISTRY,
    )
    write(root / "research/beta.md", document(schema="`inherited`, `own`").replace("alpha", "beta"))
    found = findings(root, "key-undocumented")
    assert [(item.agent, item.detail, item.path) for item in found] == [
        ("alpha", "own", PARSERS + "/alpha.py")
    ]
