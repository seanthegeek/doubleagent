"""Tests for tools/reflow_markdown.py, the Markdown paragraph reflow tool, on synthetic files."""

from __future__ import annotations

import importlib.util
import os
import random
import sys
import textwrap
import unittest
from pathlib import Path

from test_errors import CAN_DENY, DENY_REASON

TOOL = Path(__file__).resolve().parents[2] / "tools" / "reflow_markdown.py"
_spec = importlib.util.spec_from_file_location("reflow_markdown", TOOL)
assert _spec is not None and _spec.loader is not None
reflow_markdown = importlib.util.module_from_spec(_spec)
sys.modules["reflow_markdown"] = reflow_markdown
_spec.loader.exec_module(reflow_markdown)

LONG = "word " * 15 + "end"
WIDTH = 20


def reflow(text: str, width: int = WIDTH) -> str:
    """Reflow `text` and assert that a second reflow changes nothing."""
    output = reflow_markdown.reflow_text(text, width)
    assert reflow_markdown.reflow_text(output, width) == output, text
    return output


def check(text: str, expected: str, width: int = WIDTH) -> None:
    """Assert that `text` reflows to `expected`, which is itself stable."""
    assert reflow(text, width) == expected, text
    assert reflow(expected, width) == expected, expected


def in_list_item(text: str) -> str:
    """Return `text` as the content of a bullet list item."""
    lines = text.split("\n")
    rest = ["  " + line if line else "" for line in lines[1:]]
    return "\n".join(["- " + lines[0], *rest])


def in_quote(text: str) -> str:
    """Return `text` inside a blockquote."""
    return "\n".join("> " + line if line else ">" for line in text.split("\n"))


def write(path: Path, text: str) -> Path:
    """Write `text` to `path` as UTF-8 bytes, so no platform changes its line ends."""
    path.write_bytes(text.encode("utf-8"))
    return path


def raises(function, *arguments) -> str:
    """Call `function` and return the message of the ReflowError it raises."""
    try:
        function(*arguments)
    except reflow_markdown.ReflowError as error:
        return str(error)
    raise AssertionError("no ReflowError from %r" % (arguments,))


# Constructs copied byte for byte, each holding lines longer than WIDTH. They are valid at top
# level, inside a list item and inside a blockquote.
PRESERVED = {
    "backtick fence": "```python\n" + LONG + "\n\n" + LONG + "\n```\n",
    "tilde fence": "~~~~ text\n" + LONG + "\n~~~\n" + LONG + "\n~~~~\n",
    "indented code": "    " + LONG + "\n\n    " + LONG + "\n",
    "atx heading": "## " + LONG + "\n",
    "setext heading equals": LONG + "\n" + LONG + "\n===\n",
    "setext heading dashes": LONG + "\n---\n",
    "thematic break": "* * * * * * * * * * * * * * *\n",
    "table": "| a | b |\n| --- | :-: |\n| " + LONG + " | " + LONG + " |\n" + LONG + "\n",
    "html pre": "<pre>\n" + LONG + "\n\n" + LONG + "\n</pre>\n",
    "html comment": "<!-- " + LONG + "\n\n" + LONG + " -->\n",
    "html processing": "<?php " + LONG + "\n" + LONG + " ?>\n",
    "html declaration": "<!DOCTYPE " + LONG + "\n" + LONG + ">\n",
    "html cdata": "<![CDATA[ " + LONG + "\n\n" + LONG + " ]]>\n",
    "html block tag": '<div class="x">\n' + LONG + "\n" + LONG + "\n</div>\n",
    "html other tag": '<custom-tag a="b">\n' + LONG + "\n" + LONG + "\n",
    "link reference": "[label]: https://example.com/a " + '"' + LONG + '"\n',
    "footnote": "[^note]: " + LONG + "\n" + LONG + "\n    " + LONG + "\n",
    "abbreviation": "*[HTML]: " + LONG + "\n",
    "admonition": '!!! note "' + LONG + '"\n    ' + LONG + "\n    " + LONG + "\n",
    "details": "??? tip\n    " + LONG + "\n",
    "open details": "???+ tip\n    " + LONG + "\n",
    "attribute list": "Short.\n{: .class " + LONG + " }\n",
    "definition list": "Term\n:   " + LONG + "\n    " + LONG + "\n",
    "dollar math": "$$\n" + LONG + "\n\n" + LONG + "\n$$\n",
    "bracket math": "\\[\n" + LONG + "\n" + LONG + "\n\\]\n",
    "jinja": "{% if x %}\n" + LONG + "\n{% endif %}\n",
    "liquid output": LONG + " {{ page.title }}\n" + LONG + "\n",
    "hugo shortcode": LONG + "\n{{< figure " + LONG + " >}}\n",
    "spanning html tag": LONG + ' <span\nclass="x">' + LONG + "</span>\n",
    "trailing br": LONG + "<br>\n" + LONG + "\n",
    "trailing br slash": LONG + " <BR />\n" + LONG + "\n",
}
TOP_LEVEL_ONLY = {
    "yaml front matter": "---\ntitle: " + LONG + "\n---\n",
    "yaml front matter dots": "---\ntitle: " + LONG + "\n...\n",
    "toml front matter": "+++\ntitle = '" + LONG + "'\n+++\n",
    "table of contents": "[TOC]\n",
}


def test_preserved_constructs_at_top_level():
    for name, text in {**PRESERVED, **TOP_LEVEL_ONLY}.items():
        assert reflow(text) == text, name


def test_preserved_constructs_in_a_list_item():
    for name, text in PRESERVED.items():
        item = in_list_item(text)
        assert reflow(item) == item, name


def test_preserved_constructs_in_a_blockquote():
    for name, text in PRESERVED.items():
        quote = in_quote(text)
        assert reflow(quote) == quote, name


def test_text_an_extension_block_interrupts_is_left_unchanged():
    for line in ("[^1]: note", "*[HTML]: markup", "{: .class }", "$$ x $$", "!!! note"):
        text = LONG + "\n" + line + "\n"
        assert reflow(text) == text, line


def test_definition_term_after_a_blank_line_is_left_unchanged():
    text = LONG + "\n\n:   " + LONG + "\n"
    assert reflow(text) == text


def test_template_tag_on_its_own_line_keeps_the_whole_paragraph():
    text = LONG + "\n{{ page.title }}\n" + LONG + "\n"
    assert reflow(text) == text


def test_front_matter_is_only_recognised_at_the_start():
    text = "Para one two three four five.\n\n---\n"
    assert reflow(text) == "Para one two three\nfour five.\n\n---\n"
    unclosed = "---\none two three four five six\n"
    assert reflow(unclosed) == "---\none two three four\nfive six\n"


def test_paragraph_fill_at_the_default_width():
    text = ("alpha " * 30).strip() + "\n"
    output = reflow_markdown.reflow_text(text)
    lines = output.split("\n")[:-1]
    assert all(len(line) <= 80 for line in lines), output
    assert len(lines[0]) > 74, output
    assert output.split() == text.split()


def test_paragraph_fill_at_a_custom_width():
    check(
        "one two three four five six seven eight nine ten\neleven\n",
        "one two three four\nfive six seven eight\nnine ten eleven\n",
    )
    check(
        "one two three four five six seven eight nine ten\n",
        "one two three four five\nsix seven eight nine ten\n",
        width=24,
    )


def test_short_lines_are_joined():
    check("one\ntwo\nthree\n", "one two three\n")


def test_bullet_item_continuation_indent():
    check(
        "- one two three four five six seven\n",
        "- one two three four\n  five six seven\n",
    )
    check(
        "* one two three four five six\n\n  second paragraph here is long\n",
        "* one two three four\n  five six\n\n  second paragraph\n  here is long\n",
    )


def test_ordered_item_continuation_indent():
    check(
        "10. one two three four five six\n",
        "10. one two three\n    four five six\n",
    )
    check(
        "1. one two three four five six\n",
        "1. one two three\n   four five six\n",
    )


def test_a_list_python_markdown_lacks_is_left_unchanged():
    text = "1) one two three four five six\n"
    assert reflow(text) == text


def test_nested_item_continuation_indent():
    check(
        "- outer one two\n  - inner one two three four five\n",
        "- outer one two\n  - inner one two\n    three four five\n",
    )


def test_blockquote_prefix_kept():
    check(
        "> one two three four five six seven\n",
        "> one two three four\n> five six seven\n",
    )
    check(
        ">one two three four five six seven\n",
        ">one two three four\n>five six seven\n",
    )


def test_nested_blockquote_prefix():
    check(
        "> > one two three four five six\n",
        "> > one two three\n> > four five six\n",
    )


def test_list_inside_quote_is_reflowed_and_quote_inside_list_is_not():
    text = "- > one two three four five\n"
    assert reflow(text) == text
    check(
        "> - one two three four five\n",
        "> - one two three\n>   four five\n",
    )


def test_lazy_continuation_lines_join_a_quoted_paragraph():
    check(
        "> one two three\nfour five six seven\n",
        "> one two three four\n> five six seven\n",
    )


def test_a_list_item_with_a_lazy_line_is_left_unchanged():
    text = "- one two three\nfour five six seven\n"
    assert reflow(text) == text


def test_a_blockquote_ends_a_table():
    text = "| a |\n| - |\n> one two three four five six\n"
    kinds = [entry[1][1] for entry in reflow_markdown.outline(text[:-1].split("\n"))]
    assert kinds == ["table", "quote", "verbatim paragraph"], kinds
    assert reflow(text) == text


def test_a_line_starting_a_block_ends_the_lazy_paragraph():
    text = "> one two\n- item\n"
    assert reflow(text) == text


def naive(text: str, width: int = WIDTH) -> list[str]:
    """Return the lines a plain greedy fill would give."""
    return textwrap.wrap(text, width, break_long_words=False, break_on_hyphens=False)


HAZARDS = [
    "#",
    "#64](x)",
    "##",
    ">",
    ">quote",
    "-",
    "+",
    "*",
    "1.",
    "7)",
    "123456789.",
    "|",
    "```",
    "~~~",
    "<b>x</b>",
    "<!--x-->",
    "[^1]",
    "[label]:",
    ":",
    ":colon",
    "!!!",
    "???",
    "{:x}",
    "*[x",
    "$$",
    "\\[",
]
LINE_HAZARDS = ["===", "---", "***", "-- | --"]


def test_line_start_hazards_are_never_at_a_line_start():
    for hazard in HAZARDS:
        text = "aaaa bbbb cccc dddd %s eeee" % hazard
        assert naive(text)[1].startswith(hazard), hazard
        output = reflow(text + "\n")
        lines = output.split("\n")[:-1]
        assert not any(line.startswith(hazard) for line in lines[1:]), (hazard, output)
        assert output.split() == text.split(), hazard


def test_whole_line_hazards_are_never_a_line():
    for hazard in LINE_HAZARDS:
        text = "aaaa bbbb cccc dddd %s" % hazard
        assert naive(text)[1] == hazard, hazard
        output = reflow(text + "\n")
        assert hazard not in output.split("\n"), (hazard, output)
        assert output.split() == text.split(), hazard


def test_hazard_moves_the_previous_word_down():
    check("aaaa bbbb cccc dddd - eeee\n", "aaaa bbbb cccc\ndddd - eeee\n")


def test_hazard_overflows_when_no_word_can_move():
    check("aaaaaaaaaaaaaaaaaaa - eeee\n", "aaaaaaaaaaaaaaaaaaa -\neeee\n")


def test_hazard_in_a_list_item_and_a_quote():
    check("- aaaa bbbb cccc dd # eeee\n", "- aaaa bbbb cccc\n  dd # eeee\n")
    check("> aaaa bbbb cccc ddd > eeee\n", "> aaaa bbbb cccc\n> ddd > eeee\n")


def test_no_trailing_backslash_from_a_join():
    check("aaaa bbbb cccc ddd\\ eeee\n", "aaaa bbbb cccc\nddd\\ eeee\n")


def test_code_spans_are_never_split():
    check("aaaa `b c d e f g h i` ffff\n", "aaaa\n`b c d e f g h i`\nffff\n")
    check("aaaa ``b ` c d e f g`` ffff\n", "aaaa\n``b ` c d e f g``\nffff\n")


def test_code_span_keeps_its_inner_spaces():
    check("aaaa `b   c` dddd eeee ffff\n", "aaaa `b   c` dddd\neeee ffff\n")


def test_a_paragraph_with_an_unmatched_backtick_is_left_unchanged():
    text = "aaaaa `bbbb cccc dddd eeee\n"
    assert reflow(text) == text


def test_link_text_breaks_but_destination_and_title_do_not():
    check(
        '[link text here](https://example.com/a "the title") after\n',
        '[link text\nhere](https://example.com/a "the title")\nafter\n',
    )


def test_link_destination_in_angle_brackets():
    check("see [x](<a b c d e f g>) and more\n", "see\n[x](<a b c d e f g>)\nand more\n")


def test_image_alt_text_and_destination_are_never_split():
    check(
        'aa ![alt text words](img.png "a long title") end\n',
        'aa\n![alt text words](img.png "a long title")\nend\n',
    )


def test_autolink_and_inline_html_are_never_split():
    # A line may not start with `<`, so the construct stays on the line before.
    check("aaaa <https://example.com/a?b=c> dddd\n", "aaaa <https://example.com/a?b=c>\ndddd\n")
    check('aaaa <span class="x y z">b</span> c\n', 'aaaa <span class="x y z">b</span>\nc\n')
    check(
        'aaaa bbbb <span class="x y z">b</span> c\n',
        'aaaa\nbbbb <span class="x y z">b</span>\nc\n',
    )


def test_a_token_longer_than_the_width_stays_whole():
    long_word = "x" * 40
    check("aa %s bb\n" % long_word, "aa\n%s\nbb\n" % long_word)


def test_hard_break_with_two_spaces():
    check(
        "one two three four five six  \nseven eight nine ten eleven\ntwelve\n",
        "one two three four five six  \nseven eight nine ten\neleven twelve\n",
    )


def test_hard_break_with_a_backslash():
    check(
        "one\ntwo\nthree four\\\nseven eight nine ten eleven\n",
        "one two\nthree four\\\nseven eight nine ten\neleven\n",
    )


def test_hard_break_line_is_kept_as_written():
    text = "a\nbbbb cccc dddd eeee ffff gggg\\\nh\n"
    assert reflow(text) == "a\nbbbb cccc dddd eeee ffff gggg\\\nh\n"


def test_trailing_spaces_on_the_last_line_are_kept():
    check("one\ntwo  \n", "one\ntwo  \n")


def test_a_paragraph_holding_a_tab_is_left_unchanged():
    for text in ("one\ttwo three four five six seven\n", "one two three four five\tsix seven\n"):
        assert reflow(text) == text, repr(text)


def test_tab_in_the_indent_leaves_the_paragraph():
    text = "-\tone two three four five six\n"
    assert reflow(text) == text


# Command line and file handling.


# Whether both renderers are installed, which rewriting needs; read before any test replaces
# the tool's `renderers`.
RENDERERS_INSTALLED = len(reflow_markdown.renderers()) == 2
CHECK_WARNING = (
    ""
    if RENDERERS_INSTALLED
    else (
        "warning: without Python-Markdown and markdown-it-py --check cannot prove the reflow "
        "renders the same: python3 -m pip install markdown markdown-it-py\n"
    )
)


def run(capsys, *arguments: str) -> tuple[int, str, str]:
    """Run the tool's main() and return its status, stdout and stderr.

    A rewrite skips the test unless both renderers are installed, since rewriting needs them.
    """
    if "--check" not in arguments and not RENDERERS_INSTALLED:
        raise unittest.SkipTest("rewriting needs Python-Markdown and markdown-it-py")
    status = reflow_markdown.main(list(arguments))
    captured = capsys.readouterr()
    return status, captured.out, captured.err


def test_a_comment_ended_by_bang_stays_raw_up_to_its_commonmark_end(tmp_path, capsys):
    """Python-Markdown ends the comment at `--!>`, CommonMark only at `-->`."""
    text = (
        "<!-- note --!>\nfirst line of text that is long enough to wrap at twenty four\n\n"
        "-->\n\nafter the comment a paragraph that is long enough to wrap here\n"
    )
    path = write(tmp_path / "a.md", text)
    status, _stdout, stderr = run(capsys, "--width", "24", str(path))
    assert (status, stderr) == (0, "")
    out = path.read_bytes().decode()
    assert out.startswith(text.split("\n\nafter")[0] + "\n\nafter the comment a\n"), out


WRAPPED = "one two three four five six seven eight nine ten eleven twelve thirteen fourteen\n"
REFLOWED = "one two three four five\nsix seven eight nine ten\neleven twelve thirteen\nfourteen\n"


def test_single_file_rewritten(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    assert run(capsys, "--width", "24", str(path)) == (0, str(path) + "\n", "")
    assert path.read_bytes() == REFLOWED.encode()


def test_unchanged_file_is_not_written_or_printed(tmp_path, capsys):
    path = write(tmp_path / "a.md", REFLOWED)
    os.utime(path, ns=(10**18, 10**18))
    assert run(capsys, "--width", "24", str(path)) == (0, "", "")
    assert path.stat().st_mtime_ns == 10**18


def test_several_files(tmp_path, capsys):
    first = write(tmp_path / "a.md", WRAPPED)
    second = write(tmp_path / "b.md", WRAPPED)
    status, stdout, _stderr = run(capsys, "--width", "24", str(first), str(second))
    assert (status, stdout) == (0, "%s\n%s\n" % (first, second))
    assert second.read_bytes() == REFLOWED.encode()


def test_glob_expanded_by_the_tool(tmp_path, capsys):
    write(tmp_path / "a.md", WRAPPED)
    (tmp_path / "sub").mkdir()
    deep = write(tmp_path / "sub" / "b.md", WRAPPED)
    status, stdout, _stderr = run(capsys, "--width", "24", str(tmp_path / "*.md"))
    assert (status, stdout) == (0, str(tmp_path / "a.md") + "\n")
    assert deep.read_bytes() == WRAPPED.encode()
    status, stdout, _stderr = run(capsys, "--width", "24", str(tmp_path / "**" / "*.md"))
    assert (status, stdout) == (0, str(deep) + "\n")


def test_a_pattern_takes_only_markdown_files(tmp_path, capsys):
    page = write(tmp_path / "a.MD", WRAPPED)
    other = write(tmp_path / "notes.txt", WRAPPED)
    status, stdout, _stderr = run(capsys, "--width", "24", str(tmp_path / "*"))
    assert (status, stdout) == (0, str(page) + "\n")
    assert other.read_bytes() == WRAPPED.encode()


def test_pattern_matching_nothing_is_an_error(tmp_path, capsys):
    status, _out, stderr = run(capsys, str(tmp_path / "*.nothing"))
    assert status == 2
    assert "no Markdown file matches the pattern" in stderr


def test_missing_file_is_an_error(tmp_path, capsys):
    status, _out, stderr = run(capsys, str(tmp_path / "absent.md"))
    assert status == 2
    assert "no such file" in stderr


def test_directory_needs_recurse(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    status, _out, stderr = run(capsys, str(tmp_path))
    assert status == 2
    assert "pass -r" in stderr
    assert path.read_bytes() == WRAPPED.encode()


def test_recurse_takes_markdown_files_sorted(tmp_path, capsys):
    (tmp_path / "z").mkdir()
    names = [tmp_path / "z" / "b.md", tmp_path / "a.markdown", tmp_path / "c.MD"]
    for name in names:
        write(name, WRAPPED)
    other = write(tmp_path / "notes.txt", WRAPPED)
    for flag in ("-r", "--recurse"):
        for name in names:
            write(name, WRAPPED)
        status, stdout, _stderr = run(capsys, "--width", "24", flag, str(tmp_path))
        expected = sorted(str(name) for name in names)
        assert (status, stdout.split()) == (0, expected), flag
    assert other.read_bytes() == WRAPPED.encode()


def test_recurse_does_not_follow_a_symlinked_directory(tmp_path, capsys):
    outside = tmp_path / "outside"
    outside.mkdir()
    target = write(outside / "b.md", WRAPPED)
    tree = tmp_path / "tree"
    tree.mkdir()
    write(tree / "a.md", WRAPPED)
    try:
        (tree / "link").symlink_to(outside, target_is_directory=True)
    except (OSError, NotImplementedError) as error:
        raise unittest.SkipTest("cannot create a symlink: %s" % error) from error
    status, stdout, _stderr = run(capsys, "--width", "24", "-r", str(tree))
    assert (status, stdout) == (0, str(tree / "a.md") + "\n")
    assert target.read_bytes() == WRAPPED.encode()


def test_file_named_twice_is_processed_once(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    status, stdout, _stderr = run(capsys, "--check", "--width", "24", str(path), str(path))
    assert status == 1
    assert stdout.count("+++ b/") == 1
    assert stdout.endswith("1 of 1 files would be reflowed\n")


def test_check_prints_a_diff_and_writes_nothing(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    clean = write(tmp_path / "b.md", REFLOWED)
    status, stdout, stderr = run(capsys, "--check", "--width", "24", str(path), str(clean))
    assert (status, stderr) == (1, CHECK_WARNING)
    assert "--- a/%s\n+++ b/%s\n" % (path, path) in stdout
    assert "-" + WRAPPED in stdout
    assert "+six seven eight nine ten\n" in stdout
    assert stdout.endswith("1 of 2 files would be reflowed\n")
    assert path.read_bytes() == WRAPPED.encode()


def test_check_of_clean_files_exits_zero(tmp_path, capsys):
    path = write(tmp_path / "a.md", REFLOWED)
    assert run(capsys, "--check", "--width", "24", str(path)) == (
        0,
        "0 of 1 files would be reflowed\n",
        CHECK_WARNING,
    )


def test_worst_status_wins(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    bad = tmp_path / "b.md"
    bad.write_bytes(b"caf\xe9\n")
    status, stdout, stderr = run(capsys, "--check", "--width", "24", str(bad), str(path))
    assert status == 2
    assert "+++ b/%s" % path in stdout
    assert "error: %s: not UTF-8" % bad in stderr
    status, stdout, stderr = run(capsys, "--width", "24", str(bad), str(path))
    assert (status, stdout) == (2, str(path) + "\n")
    assert path.read_bytes() == REFLOWED.encode()


def test_width_below_the_minimum_is_rejected(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    try:
        reflow_markdown.main(["--width", "19", str(path)])
    except SystemExit as error:
        assert error.code == 2
    else:
        raise AssertionError("--width 19 was accepted")
    assert "at least 20" in capsys.readouterr().err


def test_crlf_file_keeps_crlf(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED.replace("\n", "\r\n") + "\r\n```\r\nx\r\n```\r\n")
    assert run(capsys, "--width", "24", str(path))[0] == 0
    expected = REFLOWED + "\n```\nx\n```\n"
    assert path.read_bytes() == expected.replace("\n", "\r\n").encode()


def test_mixed_line_ends_are_refused(tmp_path, capsys):
    for text in (WRAPPED + "a\r\nb\n", WRAPPED + "a\rb\n"):
        path = write(tmp_path / "a.md", text)
        status, _out, stderr = run(capsys, str(path))
        assert status == 2, repr(text)
        assert "mixed line ends" in stderr, repr(text)
        assert path.read_bytes() == text.encode(), repr(text)


def test_no_final_newline_is_kept(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED.rstrip("\n"))
    assert run(capsys, "--width", "24", str(path))[0] == 0
    assert path.read_bytes() == REFLOWED.rstrip("\n").encode()


def test_byte_order_mark_is_kept(tmp_path, capsys):
    path = write(tmp_path / "a.md", "\ufeff---\ntitle: x\n---\n" + WRAPPED)
    assert run(capsys, "--width", "24", str(path))[0] == 0
    assert path.read_bytes() == ("\ufeff---\ntitle: x\n---\n" + REFLOWED).encode()


def test_non_utf8_file_is_refused(tmp_path, capsys):
    path = tmp_path / "a.md"
    path.write_bytes(b"one two\xff three four five six seven eight nine ten eleven\n")
    status, _out, stderr = run(capsys, str(path))
    assert status == 2
    assert "not UTF-8" in stderr
    assert path.read_bytes().startswith(b"one two\xff")


def test_empty_file(tmp_path, capsys):
    path = write(tmp_path / "a.md", "")
    assert run(capsys, "--check", str(path))[0] == 0


def test_file_mode_is_kept(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    path.chmod(0o640)
    mode = path.stat().st_mode
    assert run(capsys, "--width", "24", str(path))[0] == 0
    assert path.stat().st_mode == mode
    assert path.read_bytes() == REFLOWED.encode()


def symlink(link: Path, target: Path) -> None:
    """Create `link` pointing at `target`, or skip the test when symlinks are unavailable."""
    try:
        link.symlink_to(target)
    except (OSError, NotImplementedError) as error:
        raise unittest.SkipTest("cannot create a symlink: %s" % error) from error


def test_symlinked_file_is_rewritten_through_to_its_target(tmp_path, capsys):
    target = write(tmp_path / "target.md", WRAPPED)
    link = tmp_path / "link.md"
    symlink(link, target)
    assert run(capsys, "--width", "24", str(link)) == (0, str(link) + "\n", "")
    assert link.is_symlink()
    assert target.read_bytes() == REFLOWED.encode()


OLD_TIME = 1_000_000_000_123_456_789


def test_keep_mtime_keeps_the_access_and_modification_times(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    os.utime(path, ns=(OLD_TIME, OLD_TIME))
    before = path.stat()
    assert run(capsys, "--keep-mtime", "--width", "24", str(path))[0] == 0
    after = path.stat()
    assert path.read_bytes() == REFLOWED.encode()
    assert (after.st_atime_ns, after.st_mtime_ns) == (before.st_atime_ns, before.st_mtime_ns)


def test_without_keep_mtime_the_modification_time_changes(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    os.utime(path, ns=(OLD_TIME, OLD_TIME))
    assert run(capsys, "--width", "24", str(path))[0] == 0
    assert path.stat().st_mtime_ns != OLD_TIME


def test_keep_mtime_through_a_symlink_keeps_the_target_times(tmp_path, capsys):
    target = write(tmp_path / "target.md", WRAPPED)
    os.utime(target, ns=(OLD_TIME, OLD_TIME))
    before = target.stat()
    link = tmp_path / "link.md"
    symlink(link, target)
    assert run(capsys, "--keep-mtime", "--width", "24", str(link))[0] == 0
    after = target.stat()
    assert target.read_bytes() == REFLOWED.encode()
    assert (after.st_atime_ns, after.st_mtime_ns) == (before.st_atime_ns, before.st_mtime_ns)


# The safety check.


def test_safety_check_refuses_a_dropped_word():
    texts = ["one two three four"]
    message = raises(reflow_markdown.check_reflow, texts, ["one two four"], WIDTH)
    assert "words of the paragraph" in message


def test_safety_check_refuses_a_changed_block():
    texts = ["para", "", "```", "code", "```"]
    doctored = ["para", "", "```", "code changed", "```"]
    message = raises(reflow_markdown.check_reflow, texts, doctored, WIDTH)
    assert message == "the fence at line 3 of the output changed"


def test_safety_check_refuses_a_new_block():
    message = raises(reflow_markdown.check_reflow, ["one", "two"], ["one", "==="], WIDTH)
    assert message == (
        "the block structure changed at line 1 of the output: a paragraph is now a heading"
    )


def test_safety_check_names_a_paragraph_that_became_unchanged():
    texts = ["one two three four five six seven <br> eight"]
    doctored = ["one two three four five six seven <br>", "eight"]
    message = raises(reflow_markdown.check_reflow, texts, doctored, WIDTH)
    assert message == (
        "the block structure changed at line 1 of the output: "
        "a paragraph is now a verbatim paragraph"
    )


def test_safety_check_refuses_an_output_that_reflows_again():
    texts = ["one two three"]
    message = raises(reflow_markdown.check_reflow, texts, ["one", "two three"], WIDTH)
    assert "again" in message


def test_safety_check_refusal_leaves_the_file_untouched(tmp_path, capsys, monkeypatch):
    path = write(tmp_path / "a.md", WRAPPED)
    original = reflow_markdown.reflow_lines

    def broken(texts, width):
        return [line.replace("seven", "") for line in original(texts, width)]

    monkeypatch.setattr(reflow_markdown, "reflow_lines", broken)
    status, stdout, stderr = run(capsys, "--width", "24", str(path))
    assert (status, stdout) == (2, "")
    assert stderr.startswith("error: %s: " % path)
    assert path.read_bytes() == WRAPPED.encode()


# Constructs only one dialect knows, and neighbours the dialects read differently.


def test_math_and_bracket_lines_keep_the_list_structure():
    for text in (
        "1. \\[Optional\\] Set up the thing\nand this is a lazy line\n2. Do the next step\n",
        "1. $$5 fee per run\nand a lazy line\n2. second step\n",
    ):
        assert reflow(text, 80) == text, text
        kinds = [entry[1][1] for entry in reflow_markdown.outline(text[:-1].split("\n"))]
        assert kinds.count("item") == 2, (text, kinds)


def test_one_dialect_lines_never_open_a_block():
    for line in ("$$", "\\[", "!!! note", "[^1]: x", "*[HTML]: y", ": def", "{: .x }"):
        text = "- one two\n  %s\n- three\n" % line
        kinds = [entry[1][1] for entry in reflow_markdown.outline(text[:-1].split("\n"))]
        assert kinds == ["item", "verbatim paragraph", "item", "paragraph"], (line, kinds)


def test_a_break_tag_is_never_left_at_a_line_end():
    text = "one two three four five six seven <br> eight nine ten eleven twelve\n"
    for width in [*range(20, 23), *range(38, 44)]:
        lines = reflow(text, width).split("\n")
        assert not any(line.endswith("<br>") for line in lines), (width, lines)


def test_an_ordered_marker_on_a_continuation_line_keeps_the_paragraph():
    text = "- Set the retry count to\n  10. Then run it again.\n"
    assert reflow(text, 80) == text


def test_a_list_right_after_a_paragraph_line_is_left_unchanged():
    text = "Text before\n- > quoted item text that is long enough to wrap here\n"
    assert reflow(text, 30) == text
    text = "Text before that is long enough to wrap\n- item\n"
    assert reflow(text, 20) == text


def test_a_setext_underline_after_a_container_line_keeps_the_paragraph():
    for text in ("> foo\n===\n", "- foo " + LONG + "\n---\n"):
        assert reflow(text) == text, text


def test_whitespace_in_a_raw_pre_body_is_kept():
    text = (
        "<div><pre>\nline one\n\nsome   preformatted   text that is long enough to wrap\n"
        "</pre></div>\n\nafter one two three four five\n"
    )
    assert reflow(text) == text


def test_whitespace_inside_inline_pre_code_and_tags_is_kept():
    check("aaaa x <pre>a  b</pre> y\n", "aaaa\nx <pre>a  b</pre> y\n")
    check('aa <span title="a>b  c">s</span> tail\n', 'aa <span title="a>b  c">s</span>\ntail\n')
    check("aa ![alt  text](i.png) tail words\n", "aa\n![alt  text](i.png)\ntail words\n")


def test_a_backtick_in_a_link_destination_keeps_the_paragraph():
    text = "word [x](foo`) and  more` other words here\n"
    assert reflow(text) == text


# The render check, when a renderer is installed.


def only_renderer(monkeypatch, name: str) -> None:
    """Make `name` the only renderer, or skip the test when it is not installed."""
    found = [entry for entry in reflow_markdown.renderers() if entry[0] == name]
    if not found:
        raise unittest.SkipTest("%s is not installed" % name)
    monkeypatch.setattr(reflow_markdown, "renderers", lambda: tuple(found))


def render_check_cases(monkeypatch, name: str) -> None:
    """Assert what the render check accepts and refuses with renderer `name` alone."""
    only_renderer(monkeypatch, name)
    difference = reflow_markdown.render_difference
    assert difference("one\ntwo   three\n", "one two three\n") == "", name
    assert difference("*one*\n", "one\n") != "", name
    assert difference("<pre>a  b</pre>\n", "<pre>a b</pre>\n") != "", name
    assert difference('<span title="a  b">x</span>\n', '<span title="a b">x</span>\n') != "", name


def test_render_check_with_python_markdown(monkeypatch):
    render_check_cases(monkeypatch, "Python-Markdown")


def test_render_check_with_markdown_it(monkeypatch):
    render_check_cases(monkeypatch, "markdown-it-py")


def test_render_difference_refuses_the_file(tmp_path, capsys, monkeypatch):
    if not reflow_markdown.renderers():
        raise unittest.SkipTest("no renderer is installed")
    path = write(tmp_path / "a.md", WRAPPED)
    monkeypatch.setattr(reflow_markdown, "render_difference", lambda before, after: "it differs")
    assert run(capsys, "--width", "24", str(path)) == (2, "", "error: %s: it differs\n" % path)
    assert path.read_bytes() == WRAPPED.encode()


def test_without_a_renderer_the_tool_still_reflows(monkeypatch):
    monkeypatch.setattr(reflow_markdown, "renderers", lambda: ())
    check("one two three four five six\n", "one two three four\nfive six\n")


# File handling the rewrite refuses.


def test_keep_mtime_keeps_distinct_access_and_modification_times(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    os.utime(path, ns=(OLD_TIME, OLD_TIME + 5 * 10**9))
    assert run(capsys, "--keep-mtime", "--width", "24", str(path))[0] == 0
    after = path.stat()
    assert (after.st_atime_ns, after.st_mtime_ns) == (OLD_TIME, OLD_TIME + 5 * 10**9)


def test_a_symlinked_directory_argument_is_refused(tmp_path, capsys):
    tree = tmp_path / "tree"
    tree.mkdir()
    target = write(tree / "a.md", WRAPPED)
    link = tmp_path / "link"
    try:
        link.symlink_to(tree, target_is_directory=True)
    except (OSError, NotImplementedError) as error:
        raise unittest.SkipTest("cannot create a symlink: %s" % error) from error
    status, _stdout, stderr = run(capsys, "-r", str(link))
    assert status == 2
    assert "symlinked directory" in stderr
    assert target.read_bytes() == WRAPPED.encode()


def test_a_file_with_other_hard_links_is_refused(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    try:
        os.link(path, tmp_path / "b.md")
    except (OSError, NotImplementedError) as error:
        raise unittest.SkipTest("cannot create a hard link: %s" % error) from error
    status, _stdout, stderr = run(capsys, "--width", "24", str(path))
    assert status == 2
    assert "hard links" in stderr
    assert path.read_bytes() == WRAPPED.encode()


def test_a_file_without_write_permission_is_refused(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED)
    if not CAN_DENY:
        raise unittest.SkipTest(DENY_REASON)
    path.chmod(0o444)
    status, _stdout, stderr = run(capsys, "--width", "24", str(path))
    path.chmod(0o644)
    assert status == 2
    assert "not writable" in stderr
    assert path.read_bytes() == WRAPPED.encode()


def test_a_fifo_is_not_a_regular_file(tmp_path, capsys):
    if not hasattr(os, "mkfifo"):
        raise unittest.SkipTest("no FIFOs on this platform")
    path = tmp_path / "pipe.md"
    os.mkfifo(path)
    status, _stdout, stderr = run(capsys, str(path))
    assert status == 2
    assert "not a regular file" in stderr


def test_check_diff_splits_lines_only_at_line_ends(tmp_path, capsys):
    path = write(tmp_path / "a.md", WRAPPED.replace("seven", "se\u2028ven"))
    status, stdout, _stderr = run(capsys, "--check", "--width", "24", str(path))
    assert status == 1
    assert "-" + WRAPPED.replace("seven", "se\u2028ven") in stdout


# Generated documents.

FUZZ_WORDS = ["alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota"]
FUZZ_SPECIAL = [
    "#", "#64", ">", ">x", "-", "+", "*", "1.", "10.", "7)", "|", "```", "~~~", "<b>x</b>",
    "<!--c-->", "[^1]", "[label]:", ":", "!!!", "???", "{:x}", "*[x", "$$", "\\[", "\\]",
    "===", "---", "`a  b`", "``c ` d``", '[link  text](http://e.com/a "t  t")',
    "![alt  text](i.png)", "<https://e.com/x>", '<span title="a>b  c">s</span>',
    "<code>a  b</code>", "<br>", "<br />", "x\\", "{x}", "`odd", "&amp;", "_em_", "**b**",
    "[ref][r]", "<pre>p  q</pre>", "{{ v }}", "2026.", "[x](foo`)", "and  more`",
]  # fmt: skip


def fuzz_words(generator, count: int) -> str:
    """Return `count` words, about one in seven of them a construct from FUZZ_SPECIAL."""
    pool = [
        generator.choice(FUZZ_SPECIAL)
        if generator.random() < 0.15
        else generator.choice(FUZZ_WORDS)
        for _ in range(count)
    ]
    return " ".join(pool)


def fuzz_paragraph(generator) -> list[str]:
    """Return the lines of a paragraph, some ending in a hard break."""
    lines = []
    for _ in range(generator.randint(1, 4)):
        line = fuzz_words(generator, generator.randint(1, 12))
        if generator.random() < 0.08:
            line += generator.choice(["  ", "\\"])
        lines.append(line)
    return lines


def fuzz_block(generator, depth: int = 0) -> list[str]:
    """Return the lines of a block: a paragraph, a list item, a quote or a fixed construct."""
    kind = generator.random()
    if kind < 0.35 or depth > 2:
        return fuzz_paragraph(generator)
    if kind < 0.5:
        marker = generator.choice(["- ", "* ", "1. ", "2) ", "10. "])
        inner = fuzz_block(generator, depth + 1)
        indent = " " * len(marker)
        lines = [marker + inner[0]]
        lines += [(indent if generator.random() < 0.8 else "") + line for line in inner[1:]]
        if generator.random() < 0.5:
            lines += [""] * generator.randint(0, 1) + [marker + fuzz_words(generator, 8)]
        return lines
    if kind < 0.6:
        inner = fuzz_block(generator, depth + 1)
        return [
            ("> " if generator.random() < 0.9 or not index else "") + line
            for index, line in enumerate(inner)
        ]
    words = fuzz_words(generator, 10)
    return generator.choice(
        [
            ["```py", words, "", "x  y", "```"],
            ["    code  " + words],
            ["## " + words],
            [words, generator.choice(["===", "---"])],
            ["| a | b |", "| - | - |", "| " + words + " | c |"],
            ["<div>", words, "</div>"],
            ["<div><pre>", "line one", "", "some   pre   text " + words, "</pre></div>"],
            ["<!-- " + words, "", "more -->"],
            ["$$", words, "$$"],
            ["!!! note", "    " + words],
            ["[^1]: " + words, "    " + words],
            ["Term", ":   " + words],
            ["{: .class }"],
            ["*[HTML]: Hyper Text"],
            ['[r]: http://e.com/r "T"'],
            ["***"],
        ]
    )


def fuzz_documents(count: int, seed: int) -> list[str]:
    """Return `count` documents built from `seed`, most blocks separated by a blank line."""
    generator = random.Random(seed)
    documents = []
    for _ in range(count):
        parts = ["---\ntitle: x\n---"] if generator.random() < 0.1 else []
        parts += ["\n".join(fuzz_block(generator)) for _ in range(generator.randint(2, 7))]
        separator = "\n\n" if generator.random() < 0.7 else "\n"
        documents.append(separator.join(parts) + "\n")
    return documents


def test_generated_documents_reflow_without_refusal():
    for number, document in enumerate(fuzz_documents(300, 306)):
        for width in (20, 33, 80):
            try:
                output = reflow_markdown.reflow_text(document, width)
            except reflow_markdown.ReflowError as error:
                message = "document %d, width %d: %s\n%s" % (number, width, error, document)
                raise AssertionError(message) from error
            again = reflow_markdown.reflow_text(output, width)
            assert again == output, "document %d, width %d is not stable" % (number, width)


# Rewriting needs both renderers; --check warns without them.


def test_rewrite_without_renderers_exits_2_and_writes_nothing(tmp_path, capsys, monkeypatch):
    path = write(tmp_path / "a.md", WRAPPED)
    monkeypatch.setattr(reflow_markdown, "renderers", lambda: ())
    status = reflow_markdown.main(["--width", "24", str(path)])
    captured = capsys.readouterr()
    assert (status, captured.out) == (2, "")
    assert captured.err.startswith("error: rewriting needs Python-Markdown and markdown-it-py")
    assert "python3 -m pip install markdown markdown-it-py" in captured.err
    assert path.read_bytes() == WRAPPED.encode()


def test_rewrite_with_one_renderer_exits_2(tmp_path, capsys, monkeypatch):
    path = write(tmp_path / "a.md", WRAPPED)
    monkeypatch.setattr(reflow_markdown, "renderers", lambda: (("markdown-it-py", str),))
    assert reflow_markdown.main(["--width", "24", str(path)]) == 2
    assert capsys.readouterr().err.startswith("error: rewriting needs Python-Markdown and")
    assert path.read_bytes() == WRAPPED.encode()


def test_check_without_renderers_warns_and_reports(tmp_path, capsys, monkeypatch):
    path = write(tmp_path / "a.md", WRAPPED)
    monkeypatch.setattr(reflow_markdown, "renderers", lambda: ())
    status, stdout, stderr = run(capsys, "--check", "--width", "24", str(path))
    assert status == 1
    assert stdout.endswith("1 of 1 files would be reflowed\n")
    assert stderr.startswith("warning: without Python-Markdown and markdown-it-py --check cannot")
    assert stderr.count("\n") == 1
    assert path.read_bytes() == WRAPPED.encode()


# A symlinked directory below the working directory is never followed.


def symlinked_tree(tmp_path, monkeypatch) -> Path:
    """Make `real/x.md` and `link -> real` in the working directory; return the page."""
    monkeypatch.chdir(tmp_path)
    (tmp_path / "real").mkdir()
    page = write(tmp_path / "real" / "x.md", WRAPPED)
    try:
        (tmp_path / "link").symlink_to("real", target_is_directory=True)
    except (OSError, NotImplementedError) as error:
        raise unittest.SkipTest("cannot create a symlink: %s" % error) from error
    return page


def test_a_pattern_skips_a_symlinked_directory(tmp_path, capsys, monkeypatch):
    symlinked_tree(tmp_path, monkeypatch)
    status, stdout, _stderr = run(capsys, "--check", "--width", "24", "**/*.md")
    assert status == 1
    assert "+++ b/real/x.md" in stdout
    assert "link" not in stdout
    assert stdout.endswith("1 of 1 files would be reflowed\n")


def test_a_pattern_skips_matched_directories_and_symlinks(tmp_path, capsys, monkeypatch):
    symlinked_tree(tmp_path, monkeypatch)
    write(tmp_path / "a.md", WRAPPED)
    for extra in ([], ["-r"]):
        status, stdout, stderr = run(capsys, "--check", "--width", "24", *extra, "*")
        assert (status, stderr) == (1, ""), extra
        assert "+++ b/a.md" in stdout and "x.md" not in stdout, extra
        assert stdout.endswith("1 of 1 files would be reflowed\n"), extra


def test_a_path_through_a_symlinked_directory_is_refused(tmp_path, capsys, monkeypatch):
    page = symlinked_tree(tmp_path, monkeypatch)
    status, stdout, stderr = run(capsys, "--check", "--width", "24", "link/x.md")
    assert status == 2
    assert "link/x.md: inside a symlinked directory" in stderr
    assert stdout == "0 of 0 files would be reflowed\n"
    assert page.read_bytes() == WRAPPED.encode()


# Parser cases.


def test_an_ordered_marker_of_ten_digits_is_a_line_start_hazard():
    text = "aaaa bbbb cccc dddd 1234567890. eeee\n"
    lines = reflow(text).split("\n")
    assert not any(line.startswith("1234567890.") for line in lines), lines


def test_a_continuation_line_with_a_long_ordered_marker_keeps_the_paragraph():
    text = "one two three\n1234567890. four five six seven eight\n"
    assert reflow(text) == text


def test_a_pre_opened_in_a_heading_keeps_later_paragraphs():
    for opener in ("# Title <pre>\n", "| a <pre> |\n| - |\n"):
        text = opener + "\nsome   spaced   words that run past the width\n"
        assert reflow(text) == text, opener


def test_a_line_starting_with_an_html_declaration_keeps_the_paragraph():
    text = "one two three four\n<!- five six seven eight\n"
    assert reflow(text) == text
