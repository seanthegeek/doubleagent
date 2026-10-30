"""Tests for tools/check_smoke_coverage.py, the tables against the smoke tests, on synthetic inputs."""

from __future__ import annotations

import pathlib
import textwrap

import check_smoke_coverage as coverage
import drift_common
from check_smoke_coverage import Assertion, RowReference, Unknown

CATALOG_TEXT = """\
# home artifacts (agent|glob relative to home)
alpha|.alpha
beta|.config/beta
# project artifacts (relative to each discovered project)
alpha|.alpha-project
project|AGENTS.md
# excluded unless --full
.alpha/cache
# credential files (flagged secret, skipped with --no-secrets)
.alpha/auth.json
# docker volumes (agent|volume name glob)
beta|*beta_data
"""

DOCUMENTS = {
    "alpha": "# alpha\n\nIntro.\n\n## Collection\n\n`~/.alpha/auth.json`, `history.jsonl` and"
    " `.alpha-project`.\n",
    "beta": "# beta\n\nIntro.\n\n## Collection\n\n`~/.config/beta/state.json`.\n",
    "shared": "# shared\n\nIntro.\n\n## Collection\n\n`AGENTS.md`.\n",
}

SMOKE_SH = r"""#!/bin/sh
WORK=$(mktemp -d)
ROOT="$WORK/root"
ALICE="$ROOT/home/alice"
mkdir -p "$ALICE/.alpha/cache" "$ROOT/srv/proj"
printf 'x\n' >"$ALICE/.alpha/auth.json"
printf 'x\n' >"$ALICE/.alpha/history.jsonl"
printf 'x\n' >"$ALICE/.config/beta/state.json"
printf '# a\n' >"$ROOT/srv/proj/AGENTS.md"
printf 'x\n' >"$ROOT/srv/proj/.alpha-project"
CASES_FW='
secret||.alpha/auth.json
'
run() { # run NAME ARGS...
  check "inside a function, never read" "row \"$ALICE/none\" | grep -q '\"secret\":true'"
}
# ---- default run ----
run default
check "alpha auth flagged" "row \"$ALICE/.alpha/auth.json\" | grep -q '\"secret\":true'"
check "alpha cache excluded" "[ \"\$(status_of \"$ALICE/.alpha/cache\")\" = skipped_excluded ]"
# ---- --no-secrets ----
run nosecrets --no-secrets
check "alpha auth skipped" "[ \"\$(status_of \"$ALICE/.alpha/auth.json\")\" = skipped_secret ]"
"""

SMOKE_PS1 = r"""$Root = Join-Path $Work 'root'
function ImagePath([string]$relativePath) { return (Join-Path $Root $relativePath) }
function IsSecretRow([string]$relativePath) { $row = Row $relativePath; return ($row -and $row.secret -eq $true -and $row.status -eq 'collected') }
MakeFile 'Users/alice/.alpha/auth.json' 'x'
MakeFile 'Users/alice/.alpha/cache/f' 'x'
MakeFile 'Users/alice/.alpha/history.jsonl' 'x'
MakeFile 'Users/alice/.config/beta/state.json' 'x'
MakeFile 'proj/AGENTS.md' '# a'
MakeFile 'proj/.alpha-project' 'x'
$CasesFrameworks = @'
secret||.alpha/auth.json
'@
# ---- default run ----
Run 'default' @{}
Check 'alpha auth flagged' { IsSecretRow 'Users/alice/.alpha/auth.json' }
Check 'alpha cache excluded' { (StatusOf 'Users/alice/.alpha/cache') -eq 'skipped_excluded' }
# ---- -NoSecrets ----
Run 'nosecrets' @{ NoSecrets = $true }
Check 'alpha auth skipped' { (StatusOf 'Users/alice/.alpha/auth.json') -eq 'skipped_secret' }
"""


def write(path: pathlib.Path, text: str) -> pathlib.Path:
    """Write `text` to `path`, creating its directory, and return the path."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def repository(root: pathlib.Path, shell: str = SMOKE_SH, powershell: str = SMOKE_PS1) -> None:
    """Write a synthetic repository: the catalog, three documents and both smoke tests."""
    write(root / "analyzer/doubleagent/catalog.txt", CATALOG_TEXT)
    for agent, text in DOCUMENTS.items():
        write(root / "research" / ("%s.md" % agent), text)
    write(root / coverage.SMOKE_SH, shell)
    write(root / coverage.SMOKE_PS1, powershell)


def findings(root: pathlib.Path) -> list[drift_common.Finding]:
    """Return the checker's findings on the repository at `root`."""
    return list(coverage.run(root))


def details(root: pathlib.Path, kind: str) -> list[str]:
    """Return the details of the findings of one kind, in run order."""
    return [finding.detail for finding in findings(root) if finding.kind == kind]


def without(text: str, line_part: str) -> str:
    """Return `text` less every line holding `line_part`."""
    return "".join(line for line in text.splitlines(True) if line_part not in line)


# --- The clean repository and each rule --------------------------------------------------------


def test_clean_repository_has_no_findings(tmp_path):
    repository(tmp_path)
    assert findings(tmp_path) == []


def test_secret_check_missing(tmp_path):
    repository(
        tmp_path, without(SMOKE_SH, "alpha auth flagged"), without(SMOKE_PS1, "auth flagged")
    )
    found = [item for item in findings(tmp_path) if item.kind == "secret-check-missing"]
    assert [item.detail for item in found] == [
        "smoke.sh .alpha/auth.json",
        "smoke.ps1 .alpha/auth.json",
    ]
    assert {item.agent for item in found} == {"alpha"}
    assert found[0].path == coverage.SMOKE_SH


def test_secret_check_in_a_no_secrets_run_does_not_count(tmp_path):
    moved = SMOKE_SH.replace(
        'check "alpha auth skipped"',
        'check "flagged here" "row \\"$ALICE/.alpha/auth.json\\" | grep -q \'\\"secret\\":true\'"\n'
        'check "alpha auth skipped"',
    )
    repository(tmp_path, without(moved, "alpha auth flagged"))
    assert details(tmp_path, "secret-check-missing") == ["smoke.sh .alpha/auth.json"]


def test_nosecrets_check_missing(tmp_path):
    repository(tmp_path, without(SMOKE_SH, "alpha auth skipped"))
    assert details(tmp_path, "nosecrets-check-missing") == ["smoke.sh .alpha/auth.json"]


def test_exclusion_check_missing(tmp_path):
    repository(tmp_path, powershell=without(SMOKE_PS1, "cache excluded"))
    assert details(tmp_path, "exclusion-check-missing") == ["smoke.ps1 .alpha/cache"]


def test_exclusion_check_in_a_full_run_does_not_count(tmp_path):
    full = SMOKE_SH.replace("run default", "run full --full")
    repository(tmp_path, full)
    assert "smoke.sh .alpha/cache" in details(tmp_path, "exclusion-check-missing")


def test_fixture_missing(tmp_path):
    repository(tmp_path, without(SMOKE_SH, ".config/beta/state.json"))
    found = [item for item in findings(tmp_path) if item.kind == "fixture-missing"]
    assert [(item.agent, item.detail) for item in found] == [("beta", "smoke.sh .config/beta")]


def test_fixture_missing_for_a_project_line(tmp_path):
    repository(tmp_path, powershell=without(SMOKE_PS1, "proj/.alpha-project"))
    assert details(tmp_path, "fixture-missing") == ["smoke.ps1 .alpha-project"]


def test_fixture_name_undocumented(tmp_path):
    extra = SMOKE_SH.replace("CASES_FW=", "printf 'x\\n' >\"$ALICE/.alpha/unnamed.db\"\nCASES_FW=")
    repository(tmp_path, extra)
    found = [item for item in findings(tmp_path) if item.kind == "fixture-name-undocumented"]
    assert [(item.agent, item.detail) for item in found] == [
        ("alpha", "smoke.sh .alpha/unnamed.db")
    ]
    assert found[0].line > 0


def test_fixture_under_an_exclusion_needs_no_name(tmp_path):
    extra = SMOKE_SH.replace(
        "CASES_FW=", "printf 'x\\n' >\"$ALICE/.alpha/cache/junk.bin\"\nCASES_FW="
    )
    repository(tmp_path, extra)
    assert details(tmp_path, "fixture-name-undocumented") == []


def test_check_by_suffix(tmp_path):
    suffixed = SMOKE_PS1.replace(
        "(StatusOf 'Users/alice/.alpha/cache')", "(StatusOf 'alpha/cache')"
    ).replace(
        "Check 'alpha auth skipped'",
        "Check 'like' { @($script:Rows | Where-Object { $_.path -like '*auth.json' }).Count -eq 1 }\n"
        "Check 'alpha auth skipped'",
    )
    repository(tmp_path, powershell=suffixed)
    assert details(tmp_path, "check-by-suffix") == ["smoke.ps1 alpha/cache", "smoke.ps1 *auth.json"]
    assert "smoke.ps1 .alpha/cache" in details(tmp_path, "exclusion-check-missing")


def test_framework_nosecrets_missing(tmp_path):
    repository(tmp_path, without(SMOKE_SH, "alpha auth skipped"))
    found = [item for item in findings(tmp_path) if item.kind == "framework-nosecrets-missing"]
    assert [(item.agent, item.detail) for item in found] == [("alpha", "smoke.sh alpha")]


def test_check_path_unresolved_once_per_variable(tmp_path):
    unresolved = SMOKE_SH.replace(
        "# ---- --no-secrets",
        'check "a" "row \\"$NOWHERE/x\\" | grep -q \'\\"secret\\":true\'"\n'
        'check "b" "row \\"$NOWHERE/y\\" | grep -q \'\\"secret\\":true\'"\n'
        "# ---- --no-secrets",
    )
    repository(tmp_path, unresolved)
    found = [item for item in findings(tmp_path) if item.kind == "check-path-unresolved"]
    assert [(item.detail, item.line > 0) for item in found] == [("smoke.sh NOWHERE", True)]


def test_check_unparsed(tmp_path):
    odd = SMOKE_SH.replace(
        "# ---- --no-secrets",
        'check "only a name"\ncheck "odd row" "row $(echo x) | grep -q y"\n# ---- --no-secrets',
    )
    odd_powershell = SMOKE_PS1.replace(
        "# ---- -NoSecrets",
        "Check 'odd status' { (StatusOf (Get-Thing)) -eq 'collected' }\n"
        "Check 'no block' $true\n# ---- -NoSecrets",
    )
    repository(tmp_path, odd, odd_powershell)
    assert details(tmp_path, "check-unparsed") == [
        "smoke.sh only a name",
        "smoke.sh odd row",
        "smoke.ps1 odd status",
        "smoke.ps1 no block",
    ]


# --- Baseline ----------------------------------------------------------------------------------


def test_stale_baseline_line_fails(tmp_path, capsys):
    repository(tmp_path)
    key = "check_smoke_coverage|secret-check-missing|smoke.sh .alpha/gone"
    write(tmp_path / "tools/drift-baseline/alpha.txt", key + "\n")
    assert coverage.main([], tmp_path) == 1
    assert "stale: remove %s" % key in capsys.readouterr().out


def test_update_baseline_removes_stale_line(tmp_path):
    repository(tmp_path, without(SMOKE_SH, "alpha auth skipped"))
    stale = "check_smoke_coverage|secret-check-missing|smoke.sh .alpha/gone\n"
    write(tmp_path / "tools/drift-baseline/alpha.txt", stale)
    assert coverage.main(["--update-baseline"], tmp_path) == 0
    text = (tmp_path / "tools/drift-baseline/alpha.txt").read_text(encoding="utf-8")
    assert "smoke.sh .alpha/gone" not in text
    assert "check_smoke_coverage|nosecrets-check-missing|smoke.sh .alpha/auth.json" in text
    assert coverage.main([], tmp_path) == 0


# --- sh reader heuristics ----------------------------------------------------------------------


def test_shell_tokens_forms():
    source = (
        'a \'b c\' "d $(e "f")" # a note\n'
        "x=1; y 2>/dev/null\n"
        "cat >f <<'EOF'\nbody line\nEOF\n"
        "z \\\n w\n"
    )
    tokens = coverage.shell_tokens(source)
    words = [token.text for token in tokens if token.kind == "word"]
    assert words == ["a", "'b c'", '"d $(e "f")"', "x=1", "y", "/dev/null", "cat", "f", "z", "w"]
    heredocs = [token.heredoc for token in tokens if token.heredoc is not None]
    assert heredocs == ["body line\n"]
    assert [token.line for token in tokens if token.text == "w"] == [7]


def test_shell_expand_forms():
    variables = {"ROOT": "@work/root", "FILE": "a/b/c.json", "EMPTY": ""}
    cases = [
        ("'$ROOT'", "$ROOT"),
        ('"$ROOT/x"', "@work/root/x"),
        ("${ROOT}y", "@work/rooty"),
        ('"${FILE%/*}"', "a/b"),
        ('"${FILE%%/*}"', "a"),
        ('"${FILE#*/}"', "b/c.json"),
        ('"${FILE##*/}"', "c.json"),
        ('"${EMPTY:-d}"', "d"),
        ('"$(dirname "$FILE")"', "a/b"),
        ('"$(row "$ROOT/x")"', coverage.ROW_MARKER + "@work/root/x"),
        ('"a \\"q\\" \\$b"', 'a "q" $b'),
        ("a\\ b", "a b"),
    ]
    for raw, expected in cases:
        assert coverage.shell_expand(raw, variables) == expected, raw
    for raw, missing in [("$NOPE", ["NOPE"]), ("$(date)", ["$(date)"]), ("$?", ["$?"])]:
        assert coverage.missing_names(coverage.shell_expand(raw, variables)) == missing, raw


def test_read_fields_forms():
    assert coverage.read_fields("a|b|c|d", "|", 3) == ["a", "b", "c|d"]
    assert coverage.read_fields("  a  b c ", None, 2) == ["a", "b c"]
    assert coverage.read_fields("a|b", "", 2) == ["a|b", ""]
    assert coverage.read_fields("", "|", 2) == ["", ""]


def test_shell_test_forms():
    assert coverage.shell_test(["-n", "x", "]"]) is True
    assert coverage.shell_test(["-z", "x", "]"]) is False
    assert coverage.shell_test(["a", "=", "a", "]"]) is True
    assert coverage.shell_test(["a", "!=", "a", "]"]) is False
    assert coverage.shell_test(["-e", "x", "]"]) is None
    assert coverage.shell_test(["-n", "\x01X\x01", "]"]) is None


def test_shell_check_assertion_forms():
    expression = (
        'row "/r/a" | grep -q \'"agent":"x".*"secret":true,"status":"collected"\''
        ' && ! row "/r/b" | grep -q \'"secret":true\''
        ' && [ "$(status_of "/r/c")" = skipped_excluded ]'
        ' && [ "$(status_of "/r/d")" != skipped_secret ]'
        ' && [ -z "$(row "/r/e")" ] && [ "$(row "/r/f" | sed s/x/y/)" = z ]'
        " && row /r/g | grep -qF 'skipped_secret'"
    )
    assertions, unparsed = coverage.shell_check_assertions(expression)
    assert not unparsed
    simple = [(item.row.path, item.secret, item.status, item.negated) for item in assertions]
    assert simple == [
        ("/r/a", True, "collected", False),
        ("/r/b", True, "", True),
        ("/r/g", None, "", False),
        ("/r/c", None, "skipped_excluded", False),
        ("/r/d", None, "skipped_secret", True),
    ]
    for rejected in ('row "/r/a" | wc -l', "status_of $(x) | cat"):
        assert coverage.shell_check_assertions(rejected)[1], rejected


def test_shell_fixture_write_forms():
    smoke = coverage.read_shell_smoke(
        textwrap.dedent(
            """\
            WORK=$(mktemp -d)
            HOME_DIR="$WORK/root/home/a"
            mkdir -p "$HOME_DIR/d1" -m 700
            printf 'x\\n' >"$HOME_DIR/f1"
            : >"$HOME_DIR/f2"
            cat >"$HOME_DIR/f3" <<'EOF'
            text
            EOF
            ln -s target "$HOME_DIR/f4"
            cp "$HOME_DIR/f1" "$HOME_DIR/f5"
            touch "$HOME_DIR/f6"
            mkfifo "$HOME_DIR/f7"
            printf 'x\\n' >"$WORK/out/not-image"
            printf 'x\\n' >/dev/null
            """
        )
    )
    names = sorted(path.rsplit("/", 1)[1] for path in smoke.fixtures)
    assert names == ["d1", "f1", "f2", "f3", "f4", "f5", "f6", "f7"]
    assert "@work/root/home/a/d1" not in smoke.files
    assert "@work/root/home/a/f3" in smoke.files


def test_shell_loops_and_case_on_a_row():
    smoke = coverage.read_shell_smoke(
        textwrap.dedent(
            """\
            WORK=$(mktemp -d)
            ROOT="$WORK/root"
            LIST='
            secret||.a/key
            excl||.a/cache
            '
            printf '%s\\n' "$LIST" | while IFS='|' read -r _kind _agent _relpath; do
              [ -n "$_relpath" ] || continue
              case "$_kind" in
                excl) mkdir -p "$ROOT/home/u/$_relpath" ;;
                *) printf 'x\\n' >"$ROOT/home/u/$_relpath" ;;
              esac
            done
            for _name in one "two words"; do printf 'x\\n' >"$ROOT/home/u/$_name"; done
            cat >"$WORK/list" <<'EOF'
            .b/c|beta
            EOF
            run default
            while IFS='|' read -r _path _agent; do
              check "flagged: $_path" "row \\"$ROOT/home/u/$_path\\" | grep -q '\\"secret\\":true'"
            done <"$WORK/list"
            printf '%s\\n' "$LIST" >"$WORK/cases"
            while IFS='|' read -r _kind _agent _relpath; do
              [ -n "$_relpath" ] || continue
              _row=$(row "$ROOT/home/u/$_relpath")
              case "$_kind" in
                secret) case "$_row" in *'"secret":true,"status":"collected"'*) pass "s: $_relpath" ;; *) fail_check "s" ;; esac ;;
                excl) case "$_row" in *'"status":"skipped_excluded"'*) pass "e: $_relpath" ;; *) fail_check "e" ;; esac ;;
              esac
            done <"$WORK/cases"
            """
        )
    )
    assert sorted(smoke.fixtures) == [
        "@work/root/home/u/.a/cache",
        "@work/root/home/u/.a/key",
        "@work/root/home/u/one",
        "@work/root/home/u/two words",
    ]
    checks = [
        (check.name, [(item.row.path, item.secret, item.status) for item in check.assertions])
        for check in smoke.checks
    ]
    assert checks == [
        ("flagged: .b/c", [("@work/root/home/u/.b/c", True, "")]),
        ("s: .a/key", [("@work/root/home/u/.a/key", True, "collected")]),
        ("e: .a/cache", [("@work/root/home/u/.a/cache", None, "skipped_excluded")]),
    ]
    assert smoke.unresolved == {}


def test_shell_while_with_unknown_data_is_unresolved():
    smoke = coverage.read_shell_smoke(
        'while read -r _x; do check "c" "row \\"$_x\\" | grep -q y"; done <"$ELSEWHERE"\n'
    )
    assert smoke.unresolved == {'"$ELSEWHERE"': 1}
    assert smoke.checks == []


def test_run_blocks_end_at_the_next_run_or_a_banner():
    smoke = coverage.read_shell_smoke(
        "run default\ncheck a b\nrun nosecrets --no-secrets\ncheck c d\n# ---- inventory\ncheck e f\n"
    )
    runs = [(run.name, run.no_secrets) for run in smoke.runs]
    assert runs == [("default", False), ("nosecrets", True)]
    assert [getattr(smoke.run_at(line), "name", None) for line in (2, 4)] == [
        "default",
        "nosecrets",
    ]
    assert smoke.run_at(6) is None


def test_shell_variable_with_a_command_substitution_is_named_when_used():
    smoke = coverage.read_shell_smoke(
        "WORK=$(mktemp -d)\nCR=$(printf '\\r')\nprintf x >\"$WORK/root/home/a/x$CR\"\n"
    )
    assert smoke.unresolved == {"CR": 3}


# --- PowerShell reader heuristics --------------------------------------------------------------


def test_powershell_tokens_forms():
    source = (
        "$a = 'it''s' # note\n"
        '$b = "x`"y$($c)"\n'
        "$h = @'\nline one\n'@\n"
        "<# block\ncomment #>\n"
        "(Row 'p').secret -eq $script:Value `\n  -and $true\n"
    )
    tokens = coverage.powershell_tokens(source)
    kinds = [(token.kind, token.text) for token in tokens if token.kind != "newline"]
    assert ("string", "it's") in kinds
    assert ("expandable", 'x`"y$($c)') in kinds
    assert ("string", "line one") in kinds
    assert ("variable", "value") in kinds
    assert ("punct", ".") in kinds and ("word", "secret") in kinds
    assert not any("note" in text or "block" in text for _, text in kinds)
    assert [token.line for token in tokens if token.text == "-and"] == [9]


def test_powershell_value_forms():
    variables = {"root": "@work/root", "list": ("a|1", "b|2"), "name": "x"}
    cases = [
        ("'Users/alice'", "Users/alice"),
        ('"Users/$name/`$y`n"', "Users/x/$y\n"),
        ("\"p/$(($list[1] -split '\\|')[0])\"", "p/b"),
        ("@('a', 'b')", ("a", "b")),
        ("@(\n  'a'\n  'b'\n)", ("a", "b")),
        ("@('a|b|c' -split '\\|', 2)", ("a", "b|c")),
        ("'a' + 'b'", "ab"),
        ("@('a') + @('b')", ("a", "b")),
        ("$list[0]", "a|1"),
        ("Join-Path $root 'x'", "@work/root/x"),
        ("(ImagePath 'Users/a')", "@work/root/Users/a"),
        ("[System.IO.Path]::Combine($root, 'Users', 'a')", "@work/root/Users/a"),
        ('@("`na`n" -split "`r?`n" | Where-Object { $_ -ne \'\' })', ("a",)),
    ]
    for text, expected in cases:
        assert coverage.powershell_value(text, variables) == expected, text
    assert coverage.powershell_value("$missing + 'x'", variables) == Unknown(("missing",))
    assert isinstance(coverage.powershell_value("(Get-Date).Year", variables), Unknown)


def test_powershell_fixture_write_forms():
    smoke = coverage.read_powershell_smoke(
        textwrap.dedent(
            """\
            $Root = Join-Path $Work 'root'
            $LinkRoot = Join-Path $Work 'linkroot'
            MakeFile 'Users/a/f1' 'x'
            MakeSizedFile "Users/a/f2" 10
            New-Item -ItemType Directory -Path (ImagePath 'Users/a/d1') -Force | Out-Null
            New-Item -ItemType SymbolicLink -Path (ImagePath 'Users/a/f3') -Target 'x' | Out-Null
            $dir = [System.IO.Path]::Combine($Root, 'Users', 'a', 'd2')
            [void][System.IO.Directory]::CreateDirectory($dir)
            foreach ($name in @('f4', 'f5')) { [System.IO.File]::WriteAllText([System.IO.Path]::Combine($dir, $name), 'x') }
            MakeLinkFile 'home/e/f6' 'x'
            MakeFile $Unset 'x'
            """
        )
    )
    assert sorted(smoke.fixtures) == [
        "@work/linkroot/home/e/f6",
        "@work/root/Users/a/d1",
        "@work/root/Users/a/d2",
        "@work/root/Users/a/d2/f4",
        "@work/root/Users/a/d2/f5",
        "@work/root/Users/a/f1",
        "@work/root/Users/a/f2",
        "@work/root/Users/a/f3",
    ]
    assert "@work/root/Users/a/d1" not in smoke.files
    assert smoke.unresolved == {"unset": 11}


def test_powershell_check_assertion_forms():
    smoke = coverage.read_powershell_smoke(
        textwrap.dedent(
            """\
            $Root = Join-Path $Work 'root'
            function IsAgentRow([string]$relativePath, [string]$agent, [bool]$secret) { $row = Row $relativePath; return ($row -and $row.agent -eq $agent -and $row.secret -eq $secret -and $row.status -eq 'collected') }
            function HasNoRow([string]$tail) { return (@($script:Rows | Where-Object { ($_.path -replace '\\\\', '/') -like "*/$tail" }).Count -eq 0) }
            Run 'default' @{}
            $row = Row 'Users/a/one'
            Check 'variable' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
            Check 'status' { (StatusOf 'Users/a/two') -eq 'skipped_excluded' -and (StatusOf "Users/a/three") -ne 'collected' }
            Check 'property' { (Row 'Users/a/four').secret -eq $false }
            Check 'in block' { $inner = Row 'Users/a/five'; $inner.status -eq 'skipped_secret' }
            Check 'helper' { IsAgentRow 'Users/a/six' 'alpha' $true }
            Check 'suffix helper' { HasNoRow 'a/seven' }
            foreach ($caseLine in @('secret||.x/key')) {
              $fields = @($caseLine -split '\\|', 3)
              switch ($fields[0]) {
                'excl' { Check "excluded: $($fields[2])" { (StatusOf "Users/a/$($fields[2])") -eq 'skipped_excluded' } }
                'secret' { Check "secret: $($fields[2])" { (Row "Users/a/$($fields[2])").secret -eq $true } }
              }
            }
            """
        )
    )
    checks = {
        check.name: [
            (item.row.path.rsplit("/", 2)[-1], item.secret, item.status, item.negated)
            for item in check.assertions
        ]
        for check in smoke.checks
    }
    assert checks == {
        "variable": [("one", True, "", False), ("one", None, "collected", False)],
        "status": [("two", None, "skipped_excluded", False), ("three", None, "collected", True)],
        "property": [("four", False, "", False)],
        "in block": [("five", None, "skipped_secret", False)],
        "helper": [("six", True, "collected", False)],
        "suffix helper": [],
        "secret: .x/key": [("key", True, "", False)],
    }
    assert smoke.suffix_patterns == [(11, "a/seven")]
    assert smoke.unparsed == []


# --- Paths, globs and spans --------------------------------------------------------------------


def test_image_path_forms():
    assert coverage.image_path("@work/root/home/a/x") == ("@work/root", "home/a/x")
    assert coverage.image_path("@work/root73/Users/b") == ("@work/root73", "Users/b")
    assert coverage.image_path("@work/linkroot/home/e") == ("@work/linkroot", "home/e")
    assert coverage.image_path("@work/out/default") is None
    assert coverage.image_path("/etc/passwd") is None


def test_collection_bases_forms():
    def claims(relative: str) -> str:
        return "alpha" if relative.split("/")[0] == ".alpha" else ""

    def bases(path: str) -> list[tuple[str, str, str]]:
        found = coverage.collection_bases("@work/root/" + path, claims)
        return [(base.kind, base.relative, base.volume) for base in found]

    assert bases("home/a/.alpha/x") == [("home", ".alpha/x", "")]
    assert bases("Users/b/Documents/app/.rules") == [
        ("home", "Documents/app/.rules", ""),
        ("project", "app/.rules", ""),
        ("project", ".rules", ""),
    ]
    assert bases("usr/share/ollama/.alpha/h") == [("home", ".alpha/h", "")]
    assert bases("root/.alpha") == [("home", ".alpha", "")]
    assert bases("srv/proj/AGENTS.md") == [("project", "AGENTS.md", "")]
    assert bases("proj/.claude/x") == [("project", ".claude/x", "")]
    assert bases("var/lib/docker/volumes/v_data/_data/.env") == [("volume", ".env", "v_data")]
    assert bases("etc/passwd") == []


def test_subtree_glob_regex_forms():
    regex = coverage.subtree_glob_regex("*/User/[!x]ache/*", case_insensitive=False)
    assert regex.fullmatch("AppData/Roaming/Code/User/Cache/a/b")
    assert not regex.fullmatch("AppData/User/xache/a")
    assert not regex.fullmatch("appdata/user/Cache/a")
    insensitive = coverage.subtree_glob_regex(".alpha/Cache", case_insensitive=True)
    assert insensitive.fullmatch(".ALPHA/cache")
    assert coverage.subtree_glob_regex("a?c", False).fullmatch("a/c")


def test_entry_claims_matches_leading_segments():
    regexes = coverage.segment_regexes(".claude-*/projects", case_insensitive=False)
    assert coverage.entry_claims(regexes, ".claude-work/projects/x/y.jsonl")
    assert not coverage.entry_claims(regexes, ".claude-work")
    assert not coverage.entry_claims(regexes, ".claude/work/projects")


def test_span_glob_forms():
    cases = [
        ("~/.alpha/auth.json", ".alpha/auth.json"),
        ("$HOME/.alpha/", ".alpha"),
        ("%APPDATA%\\Alpha\\x.json", "Alpha/x.json"),
        ("./AGENTS.md", "AGENTS.md"),
        ("projects/<slug>/{uuid}.jsonl", "projects/*/*.jsonl"),
        ("sessions/YYYY/MM/DD/x.jsonl", "sessions/*/*/*/x.jsonl"),
        ("MMX/YYYY.log", "MMX/YYYY.log"),
    ]
    for span, expected in cases:
        assert coverage.span_glob(span) == expected, span


def test_span_names_path_forms():
    path = ".alpha/projects/-srv-proj/s1.jsonl"
    for span in (
        "s1.jsonl",
        "projects/<slug>/<id>.jsonl",
        "~/.alpha/projects/*/s1.jsonl",
        "*.jsonl",
    ):
        assert coverage.span_names_path(span, path), span
    for span in ("projects", "*", "<id>", "other/s1.jsonl", "[n-m]"):
        assert not coverage.span_names_path(span, path), span
    assert coverage.span_names_path("Login Data", "Default/Login Data")
    assert coverage.span_names_path("S1.JSONL", path, case_insensitive=True)
    assert not coverage.span_names_path("S1.JSONL", path)


def test_suffix_rows_only_in_powershell():
    smoke = coverage.Smoke(coverage.SMOKE_PS1, case_insensitive=True)
    smoke.add_fixture("@work/root/Users/a/x", 1, True)
    full = RowReference("@work/root/Users/a/x", "'Users/a/x'")
    suffix = RowReference("@work/root/a/x", "'a/x'")
    smoke.rows = [(2, full), (3, suffix)]
    assert coverage.suffix_rows(smoke) == {suffix}
    smoke.case_insensitive = False
    assert coverage.suffix_rows(smoke) == set()


def test_assertion_from_text_forms():
    row = RowReference("/p", "/p")
    assert coverage.assertion_from_text(row, '"secret":true,"status":"collected"') == Assertion(
        row, True, "collected"
    )
    assert coverage.assertion_from_text(row, '"secret":false') == Assertion(row, False, "")
    assert coverage.assertion_from_text(row, "x", negated=True) == Assertion(row, None, "", True)


def test_framework_secrets_reads_the_case_list():
    smoke = coverage.Smoke(coverage.SMOKE_SH, case_insensitive=False)
    smoke.values["CASES_FW"] = "\nsecret||.alpha/a\nagent|alpha|.alpha/b\nsecret||.other/c\n"

    def claims(relative: str) -> str:
        return "alpha" if relative.startswith(".alpha/") else ""

    assert coverage.framework_secrets(smoke, claims) == {"alpha": {".alpha/a"}, "-": {".other/c"}}


def test_powershell_if_runs_a_decided_branch_or_every_branch():
    smoke = coverage.read_powershell_smoke(
        textwrap.dedent(
            """\
            $Root = Join-Path $Work 'root'
            $kind = 'b'
            if ($kind -eq 'a') { MakeFile 'Users/u/a' 'x' } else { MakeFile 'Users/u/b' 'x' }
            if ($unknown) { MakeFile 'Users/u/c' 'x' } else { MakeFile 'Users/u/d' 'x' }
            try { MakeFile 'Users/u/e' 'x' } catch { MakeFile 'Users/u/f' 'x' }
            """
        )
    )
    assert sorted(path.rsplit("/", 1)[1] for path in smoke.fixtures) == ["b", "c", "d", "e", "f"]


def test_powershell_run_flags_and_blocks():
    smoke = coverage.read_powershell_smoke(
        "Run 'default' @{}\nRun 'nosecrets' @{ NoSecrets = $true }\n"
        "Run 'full' @{ NoDocker = $true; Full = $true }\n# ---- -Inventory\nCheck 'x' { $true }\n"
    )
    runs = [(run.name, run.no_secrets, run.full) for run in smoke.runs]
    assert runs == [("default", False, False), ("nosecrets", True, False), ("full", False, True)]
    assert smoke.run_at(5) is None
