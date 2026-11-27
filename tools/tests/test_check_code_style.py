"""Tests of the Go rules of `tools/check_code_style.py` on synthetic Go source."""

from __future__ import annotations

import pathlib
import tempfile

import check_code_style as style
import pytest


def findings(source: str, name: str = "sample.go") -> list[tuple[int, str]]:
    """Return the (line, message) findings of Go `source` as a file called `name`."""
    return sorted(set(style.check_go(pathlib.Path(name), source)))


def bad_names(source: str, name: str = "sample.go") -> list[str]:
    """Return the names the name rule rejects in Go `source`, in line order."""
    return [message.split("'")[1] for _, message in findings(source, name) if "name" in message]


DECLARED_NAME_CASES = [
    ("var ab int\n", ["ab"]),
    ("var good, ab int\n", ["ab"]),
    ("const (\n\tfirst = 1\n\tab = 2\n)\n", ["ab"]),
    ("var (\n\tfirst = foo(\n\t\tab,\n\t)\n)\n", []),
    ("func main() {\n\tab := 1\n\tfirst, xy := 2, 3\n}\n", ["ab", "xy"]),
    ("func main() {\n\tfor i, item := range items {\n\t}\n}\n", []),
    ("func main() {\n\tif value, ok := table[key]; ok {\n\t}\n}\n", []),
    ("func main() {\n\tswitch kind := value.(type) {\n\t}\n}\n", []),
    ("func main() {\n\tn, k, j := 1, 2, 3\n}\n", []),
    ("type Pair struct{}\ntype ab struct{}\n", ["ab"]),
    ("type (\n\tFirst int\n\tab int\n)\n", ["ab"]),
    ("func doThing() {}\nfunc ab() {}\n", ["ab"]),
    ("func (a Stager) Name() {}\n", []),
    ("func (stager Stager) Name() {}\n", []),
    ("func (st Stager) Name() {}\n", []),
    ("func (abc Stager) Name() {}\n", []),
    ("func (stager Stager) ab() {}\n", ["ab"]),
    ("func (list *List[T]) Add(item T) {}\n", []),
]


def test_declared_names_are_checked() -> None:
    for source, expected in DECLARED_NAME_CASES:
        assert bad_names("package p\n\n" + source) == expected, source


PARAMETER_NAME_CASES = [
    ("func run(path string, mode int) {}\n", []),
    ("func run(pa string, mode int) {}\n", ["pa"]),
    ("func run(first, ab string) {}\n", ["ab"]),
    ("func run(string, int) {}\n", []),
    ("func run(path string, ab ...int) {}\n", ["ab"]),
    ("func run(path string, xy []byte) {}\n", ["xy"]),
    ("func run(path string, xy [4]byte) {}\n", ["xy"]),
    ("func run(path string) (xy int, err error) {}\n", ["xy"]),
    ("func run(path string) (int, error) {}\n", []),
    ("func run(file os.File, pkg.Value) {}\n", []),
    ("func run(handler func(ab int) error) {}\n", ["ab"]),
    ("func run() {\n\tcallback := func(ab int) {}\n}\n", ["ab"]),
    ("func run(ctx context.Context, err error) {}\n", []),
    ("func run(path string, ok bool) {}\n", []),
    ("func run(i int, j, k, n int) {}\n", []),
    ("func run(w io.Writer, r io.Reader) {}\n", []),
    ("func run(w string) {}\n", ["w"]),
    ("func run(r io.Writer) {}\n", ["r"]),
    ("func run(w http.ResponseWriter) {}\n", ["w"]),
    ("func run(t *testing.T) {}\n", ["t"]),
    ("func run(b []byte) {}\n", ["b"]),
]


def test_parameters_and_results_are_checked() -> None:
    for source, expected in PARAMETER_NAME_CASES:
        assert bad_names("package p\n\n" + source) == expected, source


def test_test_files_may_name_t_and_b() -> None:
    source = "package p\n\nfunc TestRun(t *testing.T) {}\nfunc BenchmarkRun(b *testing.B) {}\n"
    assert bad_names(source, "run_test.go") == []
    assert bad_names(source, "run.go") == ["t", "b"]


FIELD_NAME_CASES = [
    ("type Row struct {\n\tPath string\n\tAt   string\n}\n", ["At"]),
    ("type Row struct {\n\tFirst, ab int\n}\n", ["ab"]),
    ('type Row struct {\n\tsync.Mutex\n\t*Embedded\n\tName string `json:"ab"`\n}\n', []),
    ("type Row struct {\n\tEmbedded\n\tab []string\n}\n", ["ab"]),
    ("type Row struct {\n\tab func(int) error\n}\n", ["ab"]),
    ("type Row struct {\n\tab struct {\n\t\txy int\n\t}\n}\n", ["ab", "xy"]),
    ("type Row struct {\n\tBase[int]\n}\n", []),
    ("type Reader interface {\n\tRead(ab []byte) (int, error)\n\tCl()\n}\n", ["ab", "Cl"]),
]


def test_fields_and_interface_methods_are_checked() -> None:
    for source, expected in FIELD_NAME_CASES:
        assert bad_names("package p\n\n" + source) == expected, source


@pytest.mark.parametrize(
    ("source", "line"),
    [
        ("// Thing is.\ntype Thing struct {\n\t/* c */ cfg int\n}\n", 5),
        ("const (\n\t/* c */ cfg = 1\n)\n", 4),
        ("// Thing is.\ntype Thing interface {\n\t/* c */ cfg()\n}\n", 5),
        ("var (\n\t/* c */ cfg = 1\n)\n", 4),
        (
            "// run runs.\nfunc run() {\n\tvar (\n\t\tgood = 1\n\t\t/* c */ cfg = 2\n\t)\n"
            "\t_, _ = good, cfg\n}\n",
            7,
        ),
    ],
    ids=["struct field", "const entry", "interface method", "var block", "var block in a function"],
)
def test_a_comment_before_a_name_on_its_line_does_not_hide_the_name(source: str, line: int) -> None:
    assert findings("package p\n\n" + source) == [(line, "name 'cfg' abbreviates a word ('cfg')")]


def test_a_capitalised_word_may_not_be_an_abbreviation() -> None:
    cases = [
        ("srcPath", "src"),
        ("ParseCfg", "cfg"),
        ("tmpDir", "tmp"),
        ("HTTPReq", "req"),
    ]
    for name, abbreviation in cases:
        messages = [message for _, message in findings("package p\n\nvar %s int\n" % name)]
        assert messages == ["name %r abbreviates a word (%r)" % (name, abbreviation)], name


def test_interface_spellings_and_whole_words_pass() -> None:
    names = ["UID", "GID", "sha256", "mtime", "errorText", "Dev", "Ino", "usageErr", "errNotFound"]
    for name in names:
        assert findings("package p\n\nvar %s int\n" % name) == [], name


def test_text_in_strings_and_comments_binds_no_name() -> None:
    source = 'package p\n\nvar text = "ab := 1; func xy() {}" + `var cd int`\n// ab := 1\n'
    assert bad_names(source) == []


def test_a_raw_string_over_several_lines_does_not_confuse_line_numbers() -> None:
    source = "package p\n\nvar text = `a\nb\n`\nvar ab int\n"
    assert findings(source) == [(6, "name 'ab' is shorter than 3 characters")]


def test_a_finding_names_its_line() -> None:
    source = "package p\n\n// run runs.\nfunc run(path string) {\n\tab := 1\n}\n"
    assert findings(source) == [(5, "name 'ab' is shorter than 3 characters")]


def comment_lines(count: int, indent: str = "") -> str:
    """Return `count` whole-line Go comments."""
    return "".join("%s// line %d of the comment\n" % (indent, number) for number in range(count))


def test_a_run_of_eleven_comment_lines_in_a_body_fails_and_ten_pass() -> None:
    head = "package p\n\n// run runs.\nfunc run() {\n"
    assert findings(head + comment_lines(10, "\t") + "\tstep()\n}\n") == []
    messages = findings(head + comment_lines(11, "\t") + "\n\tstep()\n}\n")
    assert messages == [(5, "comment block of 11 lines is longer than 10")]


def test_a_blank_line_ends_a_comment_run() -> None:
    source = "package p\n\n" + comment_lines(6) + "\n" + comment_lines(6) + "var value = 1\n"
    assert findings(source) == []


def test_a_doc_comment_of_eleven_lines_fails_and_ten_pass() -> None:
    assert findings("package p\n\n" + comment_lines(10) + "func Run() {}\n") == []
    messages = findings("package p\n\n" + comment_lines(11) + "func Run() {}\n")
    assert messages == [(3, "doc comment of 11 lines is longer than 10")]


def test_a_doc_comment_above_a_field_is_counted() -> None:
    field = "package p\n\ntype Row struct {\n" + comment_lines(11, "\t") + "\tPath string\n}\n"
    assert findings(field) == [(4, "doc comment of 11 lines is longer than 10")]


def test_a_package_comment_in_doc_go_may_run_to_eighty_lines() -> None:
    assert findings(comment_lines(80) + "package p\n", "doc.go") == []
    long_comment = comment_lines(81) + "package p\n"
    assert findings(long_comment, "doc.go") == [(1, "doc comment of 81 lines is longer than 80")]
    build = "//go:build linux\n\n" + comment_lines(40) + "package p\n"
    assert findings(build, "internal/walk/doc.go") == []


def test_a_package_comment_outside_doc_go_fails_once() -> None:
    assert findings(comment_lines(3) + "package p\n", "x.go") == [
        (1, "a package comment belongs in doc.go")
    ]
    long_comment = (
        "package comment of 31 lines is longer than 30; a package comment belongs in doc.go"
    )
    assert findings(comment_lines(31) + "package p\n", "x.go") == [(1, long_comment)]


def test_a_test_file_keeps_a_package_comment_of_thirty_lines() -> None:
    assert findings(comment_lines(30) + "package p_test\n", "x_test.go") == []
    package = comment_lines(31) + "package p_test\n"
    assert findings(package, "x_test.go") == [(1, "doc comment of 31 lines is longer than 30")]


def test_doc_go_holds_nothing_but_the_package_comment() -> None:
    with_function = comment_lines(3) + "package p\n\n// Run runs.\nfunc Run() {}\n"
    message = "doc.go holds only the package comment and the package clause"
    assert findings(with_function, "doc.go") == [(7, message)]
    with_import = comment_lines(3) + 'package p\n\nimport "os"\n'
    assert findings(with_import, "doc.go") == [(6, message)]
    assert findings(comment_lines(3) + "package p\n\n// trailing note\n", "doc.go") == []
    assert findings(with_function, "x.go") == [(1, "a package comment belongs in doc.go")]


def test_a_file_without_a_package_comment_passes() -> None:
    assert findings("package p\n\n// Run runs.\nfunc Run() {}\n", "x.go") == []
    assert findings("package p\n", "doc.go") == []


def test_a_file_header_over_ten_lines_is_exempt() -> None:
    assert findings(comment_lines(14) + "\npackage p\n") == []


def test_a_block_comment_counts_each_of_its_lines() -> None:
    body = "/*\n" + "text\n" * 12 + "*/\n"
    messages = findings("package p\n\n" + body + "func Run() {}\n")
    assert [message for _, message in messages] == ["doc comment of 14 lines is longer than 10"]


def test_a_trailing_comment_is_not_a_whole_line_comment() -> None:
    source = "package p\n\nvar values = []int{\n" + "\t1, // one\n" * 20 + "}\n"
    assert findings(source) == []


def test_a_comment_naming_history_fails() -> None:
    texts = [
        "see #516",
        "fixed in issue 12",
        "added 2026-10-04",
        "since October 2026",
        "built in 2026",
    ]
    for text in texts:
        source = "package p\n\n// %s\nfunc Run() {}\n" % text
        assert [line for line, _ in findings(source)] == [3], text


def test_history_in_a_string_or_a_copyright_line_passes() -> None:
    source = '// Copyright 2026 Someone\n\npackage p\n\nvar date = "2026-10-04 #516"\n'
    assert findings(source) == []


def test_language_of_reads_go_files() -> None:
    assert style.language_of(pathlib.Path("internal/walk/files.go")) == "go"


def test_check_file_reports_a_go_file_with_its_path() -> None:
    with tempfile.TemporaryDirectory() as directory:
        path = pathlib.Path(directory) / "bad.go"
        path.write_text("package p\n\nvar ab int\n", encoding="utf-8")
        assert style.check_file(path) == ["%s:3: name 'ab' is shorter than 3 characters" % path]


def test_the_vendored_modules_are_not_checked() -> None:
    assert not [
        path for path in style.code_files() if "vendor" in path.relative_to(style.ROOT).parts
    ]


FORMAT_CASES = [
    ('text := fmt.Sprintf("%s: %s", agent, path)', ["Sprintf"]),
    ('text := fmt.Sprintf("%s: "+\n\t"%s", agent, path)', ["Sprintf"]),
    ('return fmt.Errorf("table %s has no rows", name)', ["Errorf"]),
    ('text := fmt.Sprintf("%d rows in %s", count, name)', []),
    ('text := fmt.Sprintf("%q", name)', []),
    ('text := fmt.Sprintf("%-14s%s", agent, path)', []),
    ('text := fmt.Sprintf("100%% of %s", name)', ["Sprintf"]),
    ('return fmt.Errorf("reading %s: %w", name, err)', []),
    ("text := fmt.Sprintf(layout, agent, path)", []),
    ('text := fmt.Sprintf("no verbs")', []),
    ('fmt.Fprintf(output, "%s\n", line)', []),
]


@pytest.mark.parametrize(("statement", "expected"), FORMAT_CASES)
def test_a_format_call_that_only_joins_strings_is_reported(
    statement: str, expected: list[str]
) -> None:
    source = "package p\n\nfunc run() {\n\t" + statement + "\n}\n"
    calls = [message.split()[0][4:] for _, message in findings(source) if "only joins" in message]
    assert calls == expected


def format_messages(body: str) -> list[str]:
    """Return the format and error-text findings of Go `body` inside a function, in line order."""
    source = "package p\n\nfunc run() {\n" + body + "\n}\n"
    words = ("only joins", "no verb", "fmt.Errorf", "drops it")
    return [message for _, message in findings(source) if any(word in message for word in words)]


NO_VERB_CASES = [
    ('\treturn fmt.Errorf("no rows")', ["fmt.Errorf has no verb to fill; use `errors.New`"]),
    (
        '\ttext := fmt.Sprintf("no " +\n\t\t"verbs")',
        ["fmt.Sprintf has no verb to fill; use the string itself"],
    ),
    ('\ttext := fmt.Sprintf("100%%")', ["fmt.Sprintf has no verb to fill; use the string itself"]),
    ('\ttext := fmt.Sprintf("%d%%", count)', []),
    ('\treturn errors.New("no rows")', []),
]


@pytest.mark.parametrize(("body", "expected"), NO_VERB_CASES)
def test_a_format_call_with_no_verb_is_reported(body: str, expected: list[str]) -> None:
    assert format_messages(body) == expected


ERRORS_NEW_FORMAT = "errors.New(fmt.Sprintf(...)) makes an error from a format; use fmt.Errorf"
ERRORS_NEW_FORMAT_CASES = [
    ('\treturn errors.New(fmt.Sprintf("table %v in %s", name, path))', [ERRORS_NEW_FORMAT]),
    ('\treturn errors.New(fmt.Sprintf("%d rows", count))', [ERRORS_NEW_FORMAT]),
    ('\treturn fmt.Errorf("table %v in %s", name, path)', []),
    ("\treturn errors.New(text)", []),
    ('\ttext := fmt.Sprintf("table %v in %s", name, path)', []),
]


@pytest.mark.parametrize(("body", "expected"), ERRORS_NEW_FORMAT_CASES)
def test_errors_new_of_a_sprintf_is_reported(body: str, expected: list[str]) -> None:
    assert format_messages(body) == expected


JOINS = "fmt.Sprintf only joins strings (every verb is %s, or %v of err); use `+`"
CONSTANT_FORMAT_CASES = [
    ('const layout = "%s: %s"\n', "fmt.Sprintf(layout, agent, path)", [JOINS]),
    (
        'const (\n\tfirst = 1\n\tlayout = "%s: " +\n\t\t"%s"\n)\n',
        "fmt.Sprintf(layout, a, b)",
        [JOINS],
    ),
    ('const layout string = "%s"\n', "fmt.Sprintf(layout, agent)", [JOINS]),
    ('const layout = "%d: %s"\n', "fmt.Sprintf(layout, count, path)", []),
    ('var layout = "%s: %s"\n', "fmt.Sprintf(layout, agent, path)", []),
    ('const layout Format = "%s: %s"\n', "fmt.Sprintf(layout, agent, path)", []),
    ('const layout = prefix + "%s"\n', "fmt.Sprintf(layout, agent)", []),
    ('const layout = "%s: %s"\n', "fmt.Sprintf(other, agent, path)", []),
]


@pytest.mark.parametrize(("declaration", "call", "expected"), CONSTANT_FORMAT_CASES)
def test_a_format_held_in_a_constant_of_the_file_is_read(
    declaration: str, call: str, expected: list[str]
) -> None:
    source = "package p\n\n" + declaration + "\nfunc run() {\n\ttext := " + call + "\n}\n"
    messages = [message for _, message in findings(source) if "only joins" in message]
    assert messages == expected


PERCENT_V_CASES = [
    ('\ttext := fmt.Sprintf("file %s (%v)", name, err)', [JOINS]),
    ('\ttext := fmt.Sprintf("(%v)", err)', [JOINS]),
    ('\ttext := fmt.Sprintf("file %s (%v)", name, count)', []),
    ('\ttext := fmt.Sprintf("file %s (%v)", name, err.Error())', []),
    ('\ttext := fmt.Sprintf("file %[1]v", err)', []),
    ('\treturn fmt.Errorf("file %s (%w)", name, err)', []),
]


@pytest.mark.parametrize(("body", "expected"), PERCENT_V_CASES)
def test_a_percent_v_of_the_error_in_sprintf_only_joins_strings(
    body: str, expected: list[str]
) -> None:
    assert format_messages(body) == expected


DROPPED = "errors.New makes an error from another error's text and drops it; wrap it with %w"
DROPPED_CAUSE_CASES = [
    # Operands that are errors by the exact rule of `go_names_an_error`.
    ('\treturn errors.New("bad (" + err.Error() + ")")', [DROPPED]),
    ("\treturn errors.New(closeErr.Error() + text)", [DROPPED]),
    ("\treturn errors.New(\n\t\tprefix +\n\t\t\terr.Error())", [DROPPED]),
    ('\treturn errors.New("bad (" + current.err.Error() + ")")', [DROPPED]),
    ('\treturn errors.New("lookup: " + errNotFound.Error())', [DROPPED]),
    ('\treturn errors.New("open: " + fs.ErrNotExist)', [DROPPED]),
    ('\treturn errors.New("bad " + current.err)', [DROPPED]),
    ("\treturn errors.New(err)", [DROPPED]),
    ('\tvar closeErr error\n\treturn errors.New("bad " + state.closeErr)', [DROPPED]),
    # Spellings alone, and an error only inside a larger expression, are not errors.
    ('\treturn errors.New("bad " + state.closeErr)', []),
    ("\treturn errors.New(describe(path, err))", []),
    ('\treturn errors.New("saw " + strconv.Itoa(errCount) + " errors")', []),
    ('\terrCount := 3\n\treturn errors.New("saw " + errCount)', []),
    ("\treturn errors.New(problems[0].Message)", []),
    ("\treturn errors.New(errorText)", []),
    ("\treturn errors.New(ctx.Err())", []),
    ('\treturn errors.New("no " + name + " table")', []),
    ('\treturn fmt.Errorf("bad (%w)", err)', []),
    ("\treturn wrap(describe(path, err), err)", []),
]


@pytest.mark.parametrize(("body", "expected"), DROPPED_CAUSE_CASES)
def test_errors_new_of_an_error_text_is_reported(body: str, expected: list[str]) -> None:
    assert format_messages(body) == expected


ERRORF_JOINS = (
    "fmt.Errorf only joins strings (every verb is %s, or %v of err); use `errors.New` and `+`,"
    " or wrap an error among the values with %w (or reason.Wrap)"
)
ERRORF_DROPPED = (
    "fmt.Errorf formats an error with a verb other than %w and drops it from the chain;"
    " wrap it with %w (or reason.Wrap)"
)
ERRORF_CASES = [
    ('\treturn fmt.Errorf("x: %s", cause)', [ERRORF_JOINS]),
    ('\treturn fmt.Errorf("read %s: %v", path, err)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("file %s (%v)", name, err)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("x: %s", err)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("x: %s", err.Error())', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("%d rows: %q", count, err.Error())', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("read: %v", current.err)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("open: %v", fs.ErrNotExist)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("open: %v", state.ErrBad)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("%v: %w", first.err, second.err)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("%s: %w", second.err.Error(), first.err)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("%v: %w", err, err)', []),
    ('\treturn fmt.Errorf("%v: %w", current.err, current.err)', []),
    ('\treturn fmt.Errorf("%w: %w", first.err, second.err)', []),
    ('\treturn fmt.Errorf("read: %w", current.err)', []),
    ('\treturn fmt.Errorf("read %s: %w", path, err)', []),
    ('\treturn fmt.Errorf("%v and %w", name, err)', []),
    ('\treturn fmt.Errorf("%d rows of %v", count, name)', []),
    ('\treturn fmt.Errorf("%[2]v: %[1]w", err, path)', []),
    ('\treturn fmt.Errorf("type %T", err)', []),
    ('\treturn fmt.Errorf("at %p", err)', []),
    ('\treturn fmt.Errorf("type %T: %v", err, err)', [ERRORF_DROPPED]),
    ('\treturn fmt.Errorf("type %T: %w", err, err)', []),
    ('\ttext := fmt.Sprintf("%d rows: %v", count, err)', []),
    ('\treturn fmt.Errorf("first: %v", failures[0])', []),
    ('\treturn fmt.Errorf("read: %v", ctx.Err())', []),
    # An error only inside a larger expression is not named by it.
    ('\treturn fmt.Errorf("read: %q", wrap(err))', []),
    ('\treturn fmt.Errorf("stage %s: %w", pathOf(err), err)', []),
    ('\treturn fmt.Errorf("%d attempts: %w", attempts(err), err)', []),
    ('\treturn fmt.Errorf("%s: %w", pathOf(err), cause)', []),
]


@pytest.mark.parametrize(("body", "expected"), ERRORF_CASES)
def test_an_errorf_that_drops_an_error_from_the_chain_is_reported(
    body: str, expected: list[str]
) -> None:
    assert format_messages(body) == expected


# A name ending in `Err` or starting with `err` and a capital is an error only where the file
# shows it: declared with the type `error`, or assigned from errors.New, errors.Join or
# fmt.Errorf.
DECLARED_ERROR_CASES = [
    ('\tvar closeErr error\n\treturn fmt.Errorf("close: %v", closeErr)', [ERRORF_DROPPED]),
    ('\tvar errClose error\n\treturn fmt.Errorf("close: %v", errClose)', [ERRORF_DROPPED]),
    ('\tvar first, closeErr error\n\treturn fmt.Errorf("close: %v", closeErr)', [ERRORF_DROPPED]),
    (
        '\tcloseErr := errors.New("closed")\n\treturn fmt.Errorf("x: %v", closeErr)',
        [ERRORF_DROPPED],
    ),
    ('\tvar errGone = errors.New("gone")\n\treturn fmt.Errorf("x: %v", errGone)', [ERRORF_DROPPED]),
    (
        '\tvar errGone = fmt.Errorf("gone %d", 1)\n\treturn fmt.Errorf("x: %s", errGone)',
        [ERRORF_DROPPED],
    ),
    (
        '\tcloseErr = errors.Join(first, second)\n\treturn fmt.Errorf("x: %v", closeErr)',
        [ERRORF_DROPPED],
    ),
    (
        '\ttype state struct{ closeErr error }\n\treturn fmt.Errorf("x: %v", current.closeErr)',
        [ERRORF_DROPPED],
    ),
    ('\tcloseErr := errors.New("closed")\n\treturn fmt.Errorf("x: %w", closeErr)', []),
    (
        '\tcloseErr := errors.New("closed")\n\treturn fmt.Errorf("x: %v: %w", closeErr, closeErr)',
        [],
    ),
    ('\tcloseErr := errors.New("closed")\n\treturn fmt.Errorf("x: %d", closeCount)', []),
    # Not shown to be an error: another call's result, a count, a message, a boolean.
    ('\tcloseErr := file.Close()\n\treturn fmt.Errorf("x: %v", closeErr)', []),
    ('\tcount, closeErr := readAll()\n\treturn fmt.Errorf("x: %v", closeErr)', []),
    ('\treturn fmt.Errorf("x: %v", closeErr)', []),
    ('\treturn fmt.Errorf("%d errors in %s", errCount, name)', []),
    ('\treturn fmt.Errorf("saw %q", errMsg)', []),
    ('\treturn fmt.Errorf("bad line %d", errLine)', []),
    ('\treturn fmt.Errorf("check failed (hasErr=%v)", hasErr)', []),
    ('\thasErr := true\n\treturn fmt.Errorf("check failed (hasErr=%v)", hasErr)', []),
    ('\tvar errCount int\n\treturn fmt.Errorf("x: %v", errCount)', []),
    (
        '\tcloseErr := errors.New("closed")\n\tif closeErr == nil {\n\t}\n\treturn fmt.Errorf("x: %v", other)',
        [],
    ),
    ('\tvar errorText string\n\treturn fmt.Errorf("x: %v", errorText)', []),
]


@pytest.mark.parametrize(("body", "expected"), DECLARED_ERROR_CASES)
def test_a_name_is_an_error_only_where_the_file_shows_it(body: str, expected: list[str]) -> None:
    assert format_messages(body) == expected


def errorf_findings(source: str) -> list[str]:
    """Return the findings of Go `source` that say a fmt.Errorf drops an error."""
    return [message for _, message in findings(source) if "drops it" in message]


def grouped_source(declarations: str, signature: str, argument: str) -> str:
    """Return a file with `declarations`, and a function with `signature` that formats `argument`."""
    return (
        "package p\n\n"
        + declarations
        + "\n// f formats.\nfunc f("
        + signature
        + ') error {\n\treturn fmt.Errorf("x: %v", '
        + argument
        + ")\n}\n"
    )


# A name in a group that shares one type is declared with it, whatever its place in the group.
GROUPED_DECLARATION_CASES = [
    ("", "readErr, writeErr error", "readErr", [ERRORF_DROPPED]),
    ("", "readErr, writeErr error", "writeErr", [ERRORF_DROPPED]),
    ("", "count int, readErr, writeErr error", "readErr", [ERRORF_DROPPED]),
    ("", "count int, readErr, writeErr error", "writeErr", [ERRORF_DROPPED]),
    ("", "readErr,\n\twriteErr error", "readErr", [ERRORF_DROPPED]),
    ("var readErr, writeErr error\n", "", "readErr", [ERRORF_DROPPED]),
    ("var readErr, writeErr error\n", "", "writeErr", [ERRORF_DROPPED]),
    ("var (\n\treadErr, writeErr error\n)\n", "", "readErr", [ERRORF_DROPPED]),
    ("var (\n\treadErr, writeErr error\n)\n", "", "writeErr", [ERRORF_DROPPED]),
    ("var (\n\tcount int\n\tfirst, readErr error\n)\n", "", "first", []),
    ("var (\n\tcount int\n\tfirst, readErr error\n)\n", "", "readErr", [ERRORF_DROPPED]),
    ("type state struct{ readErr, writeErr error }\n", "", "current.readErr", [ERRORF_DROPPED]),
    ("type state struct{ readErr, writeErr error }\n", "", "current.writeErr", [ERRORF_DROPPED]),
    (
        "type state struct {\n\treadErr, writeErr error\n}\n",
        "",
        "current.readErr",
        [ERRORF_DROPPED],
    ),
    # A group of another type, and a name of an earlier group, are not errors.
    ("var readErr, count int\n", "", "readErr", []),
    ("var count, readErr int\n", "", "readErr", []),
    ("", "readErr, n int, e error", "readErr", []),
    ("", "readErr int, closeErr error", "readErr", []),
    ("", "readErr int, closeErr error", "closeErr", [ERRORF_DROPPED]),
    # `e` does not end in `Err` or start with `err` and a capital, so it is never taken as an error.
    ("", "readErr, n int, e error", "e", []),
]


@pytest.mark.parametrize(
    ("declarations", "signature", "argument", "expected"), GROUPED_DECLARATION_CASES
)
def test_every_name_of_a_group_declared_with_the_type_error_is_an_error(
    declarations: str, signature: str, argument: str, expected: list[str]
) -> None:
    assert errorf_findings(grouped_source(declarations, signature, argument)) == expected


GROUPED_ASSIGNMENT_CASES = [
    (
        '\tcount, readErr := 1, errors.New("x")\n\treturn fmt.Errorf("x: %v", readErr)',
        [ERRORF_DROPPED],
    ),
    (
        '\treadErr, count := errors.New("x"), 1\n\treturn fmt.Errorf("x: %v", readErr)',
        [ERRORF_DROPPED],
    ),
    (
        '\tfirst, bErr = x, fmt.Errorf("y %d", 1)\n\treturn fmt.Errorf("x: %v", bErr)',
        [ERRORF_DROPPED],
    ),
    (
        '\tbErr, first = fmt.Errorf("y %d", 1), x\n\treturn fmt.Errorf("x: %v", bErr)',
        [ERRORF_DROPPED],
    ),
    (
        '\tvar count, readErr = 1, errors.New("x")\n\treturn fmt.Errorf("x: %v", readErr)',
        [ERRORF_DROPPED],
    ),
    (
        '\tcount, readErr :=\n\t\t1,\n\t\terrors.New("x")\n\treturn fmt.Errorf("x: %v", readErr)',
        [ERRORF_DROPPED],
    ),
    (
        '\tstate.count, readErr = 1, errors.New("x")\n\treturn fmt.Errorf("x: %v", readErr)',
        [ERRORF_DROPPED],
    ),
    # The names are paired by position: the error is not the name's value.
    ('\treadErr, count := 1, errors.New("x")\n\treturn fmt.Errorf("x: %v", readErr)', []),
    ('\tcount, readErr := 1, errors.New("x")\n\treturn fmt.Errorf("x: %v", count)', []),
    # With a different number of values than names the pairing cannot be read from the tokens.
    ('\tcount, readErr := load(errors.New("x"))\n\treturn fmt.Errorf("x: %v", readErr)', []),
    ('\tcount, readErr := readAll()\n\treturn fmt.Errorf("x: %v", readErr)', []),
    ('\tcount, readErr := 1, file.Close()\n\treturn fmt.Errorf("x: %v", readErr)', []),
    ('\tx, hasErr := 1, true\n\treturn fmt.Errorf("x: %v", hasErr)', []),
    (
        '\tif x, readErr := 1, errors.New("x"); x > 0 {\n\t}\n\treturn fmt.Errorf("%v", readErr)',
        [ERRORF_DROPPED],
    ),
]


@pytest.mark.parametrize(("body", "expected"), GROUPED_ASSIGNMENT_CASES)
def test_names_and_values_of_one_assignment_are_paired_by_position(
    body: str, expected: list[str]
) -> None:
    assert format_messages(body) == expected


SENTINEL_CASES = [
    ('\tvar ErrGone = errors.New("gone")\n\treturn fmt.Errorf("x: %v", ErrGone)', [ERRORF_DROPPED]),
    (
        '\tvar ErrGone = fmt.Errorf("gone %d", 1)\n\treturn fmt.Errorf("x: %s", ErrGone)',
        [ERRORF_DROPPED],
    ),
    ('\tvar ErrGone error\n\treturn fmt.Errorf("x: %v", ErrGone)', [ERRORF_DROPPED]),
    (
        '\tvar (\n\t\tErrGone = errors.New("gone")\n\t)\n\treturn fmt.Errorf("x: %v", ErrGone)',
        [ERRORF_DROPPED],
    ),
    ('\tvar First, ErrGone error\n\treturn fmt.Errorf("x: %v", ErrGone)', [ERRORF_DROPPED]),
    ('\tvar ErrGone = errors.New("gone")\n\treturn fmt.Errorf("x: %w", ErrGone)', []),
    ('\tvar ErrGone = errors.New("gone")\n\treturn fmt.Errorf("x: %v: %w", ErrGone, ErrGone)', []),
    # A bare name the file does not declare as an error may be anything.
    ('\treturn fmt.Errorf("x: %v", ErrGone)', []),
    ('\tvar ErrLimit = 3\n\treturn fmt.Errorf("x: %v", ErrLimit)', []),
    ('\tvar ErrGone = file.Close()\n\treturn fmt.Errorf("x: %v", ErrGone)', []),
    # A qualified name still counts without a declaration.
    ('\treturn fmt.Errorf("x: %v", other.ErrGone)', [ERRORF_DROPPED]),
]


@pytest.mark.parametrize(("body", "expected"), SENTINEL_CASES)
def test_a_bare_exported_error_name_counts_only_where_the_file_declares_it(
    body: str, expected: list[str]
) -> None:
    assert format_messages(body) == expected


# A width or precision taken from an argument shifts the pairing of verbs and arguments, and an
# explicit index reorders it; such a format is left alone.
UNPAIRED_FORMAT_CASES = [
    '\treturn fmt.Errorf("%*d: %v", width, count, err)',
    '\treturn fmt.Errorf("%.*f: %v", digits, value, err)',
    '\treturn fmt.Errorf("%-*s %v", width, name, err)',
    '\treturn fmt.Errorf("%[2]v: %[1]v", err, path)',
    '\treturn fmt.Errorf("%*d rows: %v", err, count, name)',
]


@pytest.mark.parametrize("body", UNPAIRED_FORMAT_CASES)
def test_a_format_with_a_starred_or_indexed_verb_is_left_alone(body: str) -> None:
    assert format_messages(body) == []


def test_following_the_errorf_advice_passes_the_dropped_cause_rule() -> None:
    assert format_messages('\treturn fmt.Errorf("x: %w", cause)') == []
    assert format_messages('\treturn errors.New("x: " + cause.Error())') == [DROPPED]


def missing_comments(source: str, name: str = "sample.go") -> list[tuple[int, str]]:
    """Return the (line, message) findings of the comment-above-a-function rule."""
    return [(line, message) for line, message in findings(source, name) if "no comment" in message]


def test_only_the_function_without_a_comment_above_fails() -> None:
    for comment in ["// run runs.", "/* run runs. */", "/*\nrun runs.\n*/"]:
        source = "package p\n\n" + comment + "\nfunc run() {}\n\nfunc step() {}\n"
        line = source.count("\n", 0, source.index("func step")) + 1
        expected = [(line, "function 'step' has no comment directly above it")]
        assert missing_comments(source) == expected, comment


def test_a_function_without_a_comment_fails() -> None:
    source = "package p\n\nvar value = 1\n\nfunc run() {}\n"
    assert missing_comments(source) == [(5, "function 'run' has no comment directly above it")]


def test_a_blank_line_between_the_comment_and_the_function_fails() -> None:
    source = "package p\n\n// run runs.\n\nfunc run() {}\n"
    assert missing_comments(source) == [(5, "function 'run' has no comment directly above it")]


def test_a_trailing_comment_on_the_line_above_is_not_a_comment_above() -> None:
    source = "package p\n\nvar value = 1 // one\nfunc run() {}\n"
    assert missing_comments(source) == [(4, "function 'run' has no comment directly above it")]


def test_a_method_needs_a_comment_too() -> None:
    source = 'package p\n\n// Row is one row.\ntype Row struct{}\n\nfunc (r Row) Name() string { return "" }\n'
    assert missing_comments(source) == [(6, "method 'Name' has no comment directly above it")]
    documented = source.replace("\nfunc (r", "\n// Name names the row.\nfunc (r")
    assert missing_comments(documented) == []


def test_a_directive_is_not_a_comment_but_is_passed_over() -> None:
    bare = "//go:build linux\n\npackage p\n\n//go:noinline\nfunc run() {}\n"
    assert missing_comments(bare) == [(6, "function 'run' has no comment directly above it")]
    documented = bare.replace("\n//go:noinline", "\n// run runs.\n//go:noinline")
    assert missing_comments(documented) == []


def test_the_functions_go_test_runs_are_exempt_in_a_test_file() -> None:
    names = ["TestRun", "TestMain", "Test", "BenchmarkRun", "FuzzRun", "ExampleRun", "Example_run"]
    for name in names:
        source = "package p\n\nfunc %s() {}\n" % name
        assert missing_comments(source, "run_test.go") == [], name
        assert len(missing_comments(source, "run.go")) == 1, name
    source = "package p\n\nfunc Testable() {}\n"
    assert len(missing_comments(source, "run_test.go")) == 1


def test_a_method_named_like_a_test_is_not_exempt() -> None:
    source = "package p\n\n// Row is one row.\ntype Row struct{}\n\nfunc (r Row) TestRun() {}\n"
    assert len(missing_comments(source, "run_test.go")) == 1


def test_main_and_init_are_exempt() -> None:
    source = "package main\n\nfunc main() {}\n\nfunc init() {}\n\nfunc run() {}\n"
    assert missing_comments(source) == [(7, "function 'run' has no comment directly above it")]


def test_a_function_literal_is_not_a_declaration() -> None:
    sources = [
        "package p\n\n// run runs.\nfunc run() {\n\tdone := func() {}\n\tdone()\n}\n",
        "package p\n\nvar handler = func() {}\n",
        "package p\n\nvar handler =\n\tfunc() {}\n",
        "package p\n\nvar (\n\thandler = func() {}\n)\n",
        "package p\n\n// Row is one row.\ntype Row struct {\n\tdone func()\n}\n",
        "package p\n\n// Handler handles.\ntype Handler func(int) error\n",
    ]
    for source in sources:
        line = source.count("\n") + 1
        expected = [(line, "function 'step' has no comment directly above it")]
        assert missing_comments(source + "func step() {}\n") == expected, source


def long_bodies(source: str, name: str = "sample.go") -> list[str]:
    """Return the function-length findings of Go `source` as a file called `name`."""
    return [message for _, message in findings(source, name) if "body of" in message]


def function_of(lines: int, header: str = "func run() {") -> str:
    """Return a Go file whose one commented function has a body of `lines` lines."""
    return "package p\n\n// run runs.\n" + header + "\n" + "\tcount++\n" * lines + "}\n"


@pytest.mark.parametrize(
    ("lines", "expected"),
    [
        (0, []),
        (40, []),
        (60, []),
        (61, ["function 'run' has a body of 61 lines, over the limit of 60"]),
        (200, ["function 'run' has a body of 200 lines, over the limit of 60"]),
    ],
)
def test_a_function_body_over_sixty_lines_is_reported(lines: int, expected: list[str]) -> None:
    assert long_bodies(function_of(lines)) == expected


@pytest.mark.parametrize(
    ("source", "expected"),
    [
        (
            function_of(61, "func (row Row) run() {"),
            ["method 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (
            function_of(61, "func run(left, right int) (result struct{ count int }) {"),
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (
            function_of(61, "func run[Item any](items []Item) interface{ Len() int } {"),
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (
            function_of(61, "func run(call func(int) int, table [3]int) (count int, err error) {"),
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        # Lines of a multi-line raw string and of nested blocks and literals are body lines.
        (
            "package p\n\n// run runs.\nfunc run() {\n\ttext := `\n" + "x\n" * 60 + "`\n}\n",
            ["function 'run' has a body of 62 lines, over the limit of 60"],
        ),
        (
            "package p\n\n// run runs.\nfunc run() {\n\tdone := func() {\n"
            + "\t\tcount++\n" * 57
            + "\t}\n\tdone()\n}\n",
            [],
        ),
        # A short function after a long literal or a body-less declaration is not charged for it.
        (
            "package p\n\n// first has no body.\nfunc first()\n\n// run runs.\nfunc run() {\n"
            + "\tcount++\n" * 61
            + "}\n",
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (
            "package p\n\n// first is short.\nfunc first() {\n\tcount++\n}\n\n// run runs.\nfunc run() {\n"
            + "\tcount++\n" * 3
            + "}\n",
            [],
        ),
    ],
)
def test_the_length_of_a_body_is_read_from_its_own_braces(source: str, expected: list[str]) -> None:
    assert long_bodies(source) == expected


LONG_RUN = "function 'run' has a body of 61 lines, over the limit of 60"


def literal_of(lines: int, opening: str, closing: str = "}") -> str:
    """Return a Go file whose package variable holds a function literal of `lines` body lines."""
    return "package p\n\n" + opening + "\n" + "\tcount++\n" * lines + closing + "\n"


@pytest.mark.parametrize(
    ("lines", "opening", "closing", "expected"),
    [
        (60, "var run = func() {", "}", []),
        (
            61,
            "var run = func() {",
            "}",
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (60, "var run = func(left, right int) (count int) {", "}", []),
        (
            61,
            "var run = func(left, right int) (count int) {",
            "}",
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (61, "var run handler = func() {", "}", []),  # not read: the type is written
        (
            61,
            "var run =\n\tfunc() {",
            "}",
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (
            61,
            "var (\n\trun = func() {",
            "}\n)",
            ["function 'run' has a body of 61 lines, over the limit of 60"],
        ),
        (60, "var (\n\trun = func() {", "}\n)", []),
        (
            61,
            "var run = func() int {",
            "\treturn 1\n}()",
            ["function 'run' has a body of 62 lines, over the limit of 60"],
        ),
        # Several names, and a parenthesised literal, are read as a single name is.
        (60, "var run, stop = func() {", "}, func() {}", []),
        (61, "var run, stop = func() {", "}, func() {}", [LONG_RUN]),
        (60, "var stop, run = func() {}, func() {", "}", []),
        (61, "var stop, run = func() {}, func() {", "}", [LONG_RUN]),
        (60, "var run = (func() {", "})", []),
        (61, "var run = (func() {", "})", [LONG_RUN]),
        (60, "var (\n\trun, stop = func() {", "}, func() {}\n)", []),
        (61, "var (\n\trun, stop = func() {", "}, func() {}\n)", [LONG_RUN]),
        (61, "var run, stop handler = func() {", "}, func() {}", []),  # a type is written
        # A comment before the name on its line does not hide the literal.
        (60, "var (\n\t/* doc */ run = func() {", "}\n)", []),
        (61, "var (\n\t/* doc */ run = func() {", "}\n)", [LONG_RUN]),
        (61, "var (\n\tstop = next\n\t/* doc */ run = func() {", "}\n)", [LONG_RUN]),
        (60, "/* doc */ var run = func() {", "}", []),
        (61, "/* doc */ var run = func() {", "}", [LONG_RUN]),
        # A literal in a call, a struct field or a function body is not a package variable's own.
        (61, "var run = wrap(func() {", "})", []),
        (61, "var run = table{handler: func() {", "}}", []),
    ],
)
def test_a_function_literal_a_package_variable_holds_is_measured(
    lines: int, opening: str, closing: str, expected: list[str]
) -> None:
    assert long_bodies(literal_of(lines, opening, closing)) == expected


@pytest.mark.parametrize(("lines", "expected"), [(60, []), (61, [LONG_RUN])])
def test_a_comment_before_func_on_its_line_does_not_hide_the_function(
    lines: int, expected: list[str]
) -> None:
    assert long_bodies(function_of(lines, "/* doc */ func run() {")) == expected


def test_a_declared_function_after_a_long_literal_is_measured_by_its_own_body() -> None:
    source = literal_of(61, "var run = func() {") + "\n// next is short.\nfunc next() {\n}\n"
    assert long_bodies(source) == ["function 'run' has a body of 61 lines, over the limit of 60"]


def test_a_test_function_is_held_to_the_same_limit() -> None:
    source = function_of(61, "func TestRun(t *testing.T) {")
    assert len(long_bodies(source, "run_test.go")) == 1


def test_a_listed_function_is_passed_over_in_its_own_file_only() -> None:
    path, name = sorted(style.GO_LONG_FUNCTIONS)[0]
    source = function_of(61, "func %s() {" % name)
    assert long_bodies(source, str(style.ROOT / path)) == []
    assert len(long_bodies(source, str(style.ROOT / "internal/other/other_test.go"))) == 1
    assert len(long_bodies(function_of(61), str(style.ROOT / path))) == 1


def test_only_the_three_reviewed_lists_are_exempt_from_the_limit() -> None:
    assert {
        ("internal/follow/confine_sites_test.go", "siteClasses"),
        ("internal/follow/follow_guard_lists_test.go", "reviewedImports"),
        ("internal/follow/follow_guard_lists_test.go", "allowedCalls"),
    } == style.GO_LONG_FUNCTIONS


@pytest.mark.parametrize(("path", "name"), sorted(style.GO_LONG_FUNCTIONS))
def test_every_listed_function_is_still_over_the_limit(path: str, name: str) -> None:
    source = (style.ROOT / path).read_text(encoding="utf-8")
    lengths = {
        function: length
        for _, function, _, length in style.go_long_functions(style.go_tokens(source))
    }
    assert lengths.get(name, 0) > style.GO_MAX_FUNCTION_LINES, path


def misnamed_tests(root: pathlib.Path, files: list[str]) -> list[str]:
    """Write empty Go `files` under `root` and return the test files the name rule rejects."""
    for name in files:
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("package p\n", encoding="utf-8")
    rejected = []
    for name in files:
        for line, message in style.go_test_file_name_findings(root / name, root):
            assert (line, message) == (1, style.GO_TEST_FILE_NAME_MESSAGE)
            rejected.append(name)
    return rejected


TEST_FILE_NAME_CASES = [
    (["internal/walk/walk.go", "internal/walk/walk_test.go"], []),
    (["internal/walk/walk.go", "internal/walk/walk_symlinks_test.go"], []),
    (["internal/walk/walk.go", "internal/walk/walk_windows_test.go"], []),
    (["internal/walk/walk.go", "internal/walk/walk_symlinks_unix_test.go"], []),
    (["internal/walk/outside_unix.go", "internal/walk/outside_test.go"], []),
    (["internal/walk/outside_unix.go", "internal/walk/outside_unix_test.go"], []),
    (["internal/walk/outside_norace.go", "internal/walk/outside_race_test.go"], []),
    (["cmd/doubleagent/main.go", "cmd/doubleagent/main_flags_test.go"], []),
    (["internal/walk/doc.go", "internal/walk/doc_test.go"], []),
    (["internal/walk/doc.go", "internal/walk/anything_test.go"], []),
    (["internal/walk/anything_test.go"], []),
    (["internal/walk/walk.go", "internal/walk/testdata/other_test.go"], []),
    (["internal/vendor/walk.go", "internal/vendor/other_test.go"], []),
    (["vendor/example/walk.go", "vendor/example/other_test.go"], []),
    (["tools/walk.go", "tools/other_test.go"], []),
    (["walk.go", "other_test.go"], []),
    (["internal/walk/walk.go", "internal/walk/walker_test.go"], ["internal/walk/walker_test.go"]),
    (["internal/walk/walk.go", "internal/walk/files_test.go"], ["internal/walk/files_test.go"]),
    (
        ["internal/walk/doc.go", "internal/walk/walk.go", "internal/walk/doc_test.go"],
        ["internal/walk/doc_test.go"],
    ),
    (
        ["internal/walk/walk_windows.go", "internal/walk/walks_windows_test.go"],
        ["internal/walk/walks_windows_test.go"],
    ),
]


@pytest.mark.parametrize(("files", "expected"), TEST_FILE_NAME_CASES)
def test_a_test_file_is_named_for_a_go_file_of_its_package(
    tmp_path: pathlib.Path, files: list[str], expected: list[str]
) -> None:
    assert misnamed_tests(tmp_path, files) == expected


def test_a_misnamed_test_file_is_reported_with_its_path(
    tmp_path: pathlib.Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    misnamed_tests(tmp_path, ["internal/walk/walk.go", "internal/walk/walker_test.go"])
    monkeypatch.setattr(style, "ROOT", tmp_path)
    assert style.check_file(tmp_path / "internal/walk/walker_test.go") == [
        "internal/walk/walker_test.go:1: " + style.GO_TEST_FILE_NAME_MESSAGE
    ]


def test_check_go_applies_the_test_file_name_rule_to_the_repository() -> None:
    path = style.ROOT / "internal/catalog/no_such_file_test.go"
    assert (1, style.GO_TEST_FILE_NAME_MESSAGE) in findings("package catalog\n", str(path))
