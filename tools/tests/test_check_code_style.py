"""Tests of the Go rules of `tools/check_code_style.py` on synthetic Go source."""

from __future__ import annotations

import pathlib
import tempfile

import check_code_style as style


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
    source = "package p\n\nfunc run(path string) {\n\tab := 1\n}\n"
    assert findings(source) == [(4, "name 'ab' is shorter than 3 characters")]


def comment_lines(count: int, indent: str = "") -> str:
    """Return `count` whole-line Go comments."""
    return "".join("%s// line %d of the comment\n" % (indent, number) for number in range(count))


def test_a_run_of_eleven_comment_lines_in_a_body_fails_and_ten_pass() -> None:
    head = "package p\n\nfunc run() {\n"
    assert findings(head + comment_lines(10, "\t") + "\tstep()\n}\n") == []
    messages = findings(head + comment_lines(11, "\t") + "\n\tstep()\n}\n")
    assert messages == [(4, "comment block of 11 lines is longer than 10")]


def test_a_blank_line_ends_a_comment_run() -> None:
    source = "package p\n\n" + comment_lines(6) + "\n" + comment_lines(6) + "var value = 1\n"
    assert findings(source) == []


def test_a_doc_comment_of_eleven_lines_fails_and_ten_pass() -> None:
    assert findings("package p\n\n" + comment_lines(10) + "func Run() {}\n") == []
    messages = findings("package p\n\n" + comment_lines(11) + "func Run() {}\n")
    assert messages == [(3, "doc comment of 11 lines is longer than 10")]


def test_a_doc_comment_above_a_field_or_package_is_counted() -> None:
    field = "package p\n\ntype Row struct {\n" + comment_lines(11, "\t") + "\tPath string\n}\n"
    assert findings(field) == [(4, "doc comment of 11 lines is longer than 10")]
    package = comment_lines(11) + "package p\n"
    assert findings(package) == [(1, "doc comment of 11 lines is longer than 10")]


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
