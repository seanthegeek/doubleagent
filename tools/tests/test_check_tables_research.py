"""Tests for tools/check_tables_research.py on synthetic repositories."""

from __future__ import annotations

import pathlib
import textwrap

import check_tables_research as checker
import drift_common
from drift_common import TableLine

CATALOG = """\
# home artifacts (agent|glob relative to home)
alpha|.alpha
alpha|.config/alpha
alpha|Library/Application Support/alpha
beta|.beta
# project artifacts (relative to each discovered project)
alpha|.alpha
project|AGENTS.md
# excluded unless --full
.alpha/cache
.beta/models
models
# credential files (flagged secret, skipped with --no-secrets)
.alpha/auth.json
.config/alpha/token.json
Library/Application Support/alpha/token.json
# docker volumes (agent|volume name glob)
# Beta's volume, mounted at
# /root/.beta
beta|*beta_data
"""

ALPHA = """\
# alpha

Alpha is a test agent.

## Collection

### Per-user storage

`~/.alpha` holds the sessions, `~/.config/alpha` the settings on Linux and
`~/Library/Application Support/alpha` on macOS. A project keeps `.alpha/`.

### Credentials

- `~/.alpha/auth.json` holds the token.
- `~/.config/alpha/token.json` holds the refresh token, on macOS in
  `~/Library/Application Support/alpha/token.json`.

### Exclusions

`cache` below `.alpha`.

### Project-local files

None.

### Where the project path is recorded

`sessions.json` key `project_dir`.

## Parsing

### Record schema

`session_id`, `project_dir` and `message.content`.
"""

BETA = """\
# beta

Beta runs in Docker.

## Collection

### Per-user storage

`~/.beta` holds everything.

#### Docker volume

The `beta_data` volume holds the home, and `models` there is excluded.

### Credentials

None.

### Exclusions

`.beta/models`.

## Parsing

Collected only.
"""

SHARED = """\
# shared

## Collection

### Project-local files

`AGENTS.md` at the project root.
"""

AGENTS_PAGE = """\
# Agents

| Agent | Status | Validated against | Notes |
| --- | --- | --- | --- |
| [`alpha`](#alpha) | parsed | source | [research](../research/alpha.md) |
| [`beta`](#beta) | collected | source | [research](../research/beta.md) |
"""


def write(path: pathlib.Path, text: str) -> pathlib.Path:
    """Write dedented `text` to `path`, creating its directory, and return the path."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(textwrap.dedent(text), encoding="utf-8")
    return path


def repository(root: pathlib.Path, **changes: str) -> pathlib.Path:
    """Write the clean synthetic repository under `root`, each named file replaced by `changes`.

    The names are `catalog`, `alpha`, `beta`, `shared` and `page`; an empty text leaves the
    file out.
    """
    files = {
        "catalog": ("analyzer/doubleagent/catalog.txt", CATALOG),
        "alpha": ("research/alpha.md", ALPHA),
        "beta": ("research/beta.md", BETA),
        "shared": ("research/shared.md", SHARED),
        "page": ("docs/agents.md", AGENTS_PAGE),
    }
    for name, (path, text) in files.items():
        text = changes.get(name, text)
        if text:
            write(root / path, text)
    write(root / "analyzer/doubleagent/parsers/__init__.py", "ALL = []\n")
    return root


def findings(root: pathlib.Path, kind: str) -> list[tuple[str, str]]:
    """Return (agent, detail) of each finding of `kind` the checker yields for `root`."""
    return [
        (finding.agent, finding.detail) for finding in checker.run(root) if finding.kind == kind
    ]


def line(glob: str, agent: str = "alpha") -> TableLine:
    """Return a home table line of `agent` for `glob`."""
    return TableLine("home", agent, glob, 1, ())


def sections_of(tmp_path: pathlib.Path, text: str) -> dict[str, drift_common.Section]:
    """Return the sections of a document written from `text`."""
    return drift_common.research_sections(write(tmp_path / "document.md", text))


# --- The clean repository and the baseline ---------------------------------------------------


def test_clean_repository_has_no_finding(tmp_path):
    assert list(checker.run(repository(tmp_path))) == []


def test_stale_baseline_entry_fails(tmp_path, capsys):
    repository(tmp_path)
    write(
        tmp_path / "tools/drift-baseline/alpha.txt",
        "check_tables_research|line-unnamed|.alpha-gone\n",
    )
    assert checker.main([], tmp_path) == 1
    output = capsys.readouterr().out
    assert "stale: remove check_tables_research|line-unnamed|.alpha-gone" in output


def test_update_baseline_removes_a_stale_entry(tmp_path):
    repository(tmp_path)
    baseline = tmp_path / "tools/drift-baseline/alpha.txt"
    write(baseline, "check_other|kind|kept\ncheck_tables_research|line-unnamed|.alpha-gone\n")
    assert checker.main(["--update-baseline"], tmp_path) == 0
    assert baseline.read_text(encoding="utf-8") == "check_other|kind|kept\n"
    assert checker.main([], tmp_path) == 0


def test_new_finding_fails_until_baselined(tmp_path, capsys):
    repository(tmp_path, catalog=CATALOG.replace("beta|.beta\n", "beta|.beta\nbeta|.gamma\n"))
    assert checker.main([], tmp_path) == 1
    assert "line-unnamed [beta]: home .gamma" in capsys.readouterr().out
    assert checker.main(["--update-baseline"], tmp_path) == 0
    baseline = tmp_path / "tools/drift-baseline/beta.txt"
    assert (
        baseline.read_text(encoding="utf-8") == "check_tables_research|line-unnamed|home .gamma\n"
    )
    assert checker.main([], tmp_path) == 0


# --- Rule 1: line-unnamed --------------------------------------------------------------------


def test_line_unnamed(tmp_path):
    assert findings(repository(tmp_path / "clean"), "line-unnamed") == []
    catalog = CATALOG.replace(
        "alpha|.alpha\nalpha|.config", "alpha|.alpha\nalpha|.alpha2\nalpha|.config"
    )
    catalog = catalog.replace("models\n#", "models\n.alpha/logs\nstray\n#")
    root = repository(tmp_path / "drift", catalog=catalog)
    assert findings(root, "line-unnamed") == [
        ("alpha", "home .alpha2"),
        ("alpha", "exclude .alpha/logs"),
    ]
    assert ("-", "stray") in findings(root, "exclusion-unreachable")


def test_one_glob_unnamed_in_two_tables_gives_two_findings(tmp_path):
    catalog = CATALOG.replace("alpha|.alpha\nproject|", "alpha|.alphaboth\nproject|")
    catalog = catalog.replace("beta|.beta\n", "beta|.beta\nalpha|.alphaboth\n")
    root = repository(tmp_path, catalog=catalog)
    assert findings(root, "line-unnamed") == [
        ("alpha", "home .alphaboth"),
        ("alpha", "project .alphaboth"),
    ]


def test_line_named_by_remainder_below_an_entry(tmp_path):
    # `cache` names `.alpha/cache` below `.alpha`; without it the line is unnamed.
    root = repository(tmp_path, alpha=ALPHA.replace("`cache` below `.alpha`.", "Caches."))
    assert findings(root, "line-unnamed") == [("alpha", "exclude .alpha/cache")]


def test_star_line_reaching_two_agents_equally_is_checked_against_both(tmp_path):
    catalog = CATALOG.replace("beta|.beta\n", "beta|.beta\nbeta|Library/Tool\nalpha|.config/Tool\n")
    catalog = catalog.replace("models\n#", "models\n*/Tool/cache\n*/Tool/logs\n#")
    alpha = ALPHA.replace("`~/.alpha` holds", "`~/.config/Tool` and `~/.alpha` hold")
    beta = BETA.replace("`~/.beta` holds", "`~/Library/Tool` and `~/.beta` hold")
    root = repository(tmp_path, catalog=catalog, alpha=alpha, beta=beta)
    found = [finding for finding in checker.run(root) if finding.kind == "line-unnamed"]
    assert [(finding.agent, finding.detail) for finding in found] == [("-", "exclude */Tool/logs")]
    assert "research/alpha.md or research/beta.md" in found[0].message


def test_attributed_line_is_named_only_by_its_owner(tmp_path):
    catalog = CATALOG.replace("models\n#", "models\n.alpha/logs\n#")
    beta = BETA.replace(
        "`~/.beta` holds everything.", "`~/.beta` holds everything, not `.alpha/logs`."
    )
    root = repository(tmp_path, catalog=catalog, beta=beta)
    assert findings(root, "line-unnamed") == [("alpha", "exclude .alpha/logs")]


def test_shared_and_project_lines_are_owned_by_shared_document(tmp_path):
    root = repository(tmp_path, shared=SHARED.replace("`AGENTS.md`", "The instructions file"))
    assert findings(root, "line-unnamed") == [("project", "project AGENTS.md")]


def test_normalise_span_forms():
    cases = {
        "~/.alpha/": ".alpha",
        "$HOME/.alpha": ".alpha",
        "${HOME}/.alpha": ".alpha",
        "./.alpha": ".alpha",
        "~\\.alpha": ".alpha",
        "%USERPROFILE%\\.alpha": ".alpha",
        "%APPDATA%\\Alpha\\auth.json": "AppData/Roaming/Alpha/auth.json",
        "%LOCALAPPDATA%\\Alpha": "AppData/Local/Alpha",
        "$XDG_CONFIG_HOME/alpha": ".config/alpha",
        "$XDG_DATA_HOME/alpha": ".local/share/alpha",
        "$XDG_STATE_HOME/alpha": ".local/state/alpha",
        "$XDG_CACHE_HOME/alpha": ".cache/alpha",
        ".alpha": ".alpha",
        "/": "/",
        "/etc/alpha": "/etc/alpha",
        "x~/alpha": "x~/alpha",
    }
    for span, normalised in cases.items():
        assert checker.normalise_span(span) == normalised, span


def test_names_glob_forms():
    accepted = [
        (".alpha", ".alpha"),
        (".claude-*/projects", ".claude-work/projects"),
        (".claude-*/projects", ".claude-*/projects"),
        ("*a0_usr", "a0_usr"),
        ("a0-launcher-*-usr", "a0-launcher-<slug>-usr"),
        ("[Cc]ache", "Cache"),
        ("mcp-tokens/*", "mcp-tokens"),
    ]
    rejected = [
        (".alpha", ".alpha/sessions"),
        (".alpha/sessions", ".alpha"),
        (".claude-*/projects", ".claude/projects"),
        ("[Cc]ache", "cache2"),
        ("mcp-tokens/*/x", "mcp-tokens"),
    ]
    for glob, span in accepted:
        assert checker.names_glob(glob, [span]), (glob, span)
    for glob, span in rejected:
        assert not checker.names_glob(glob, [span]), (glob, span)


def test_names_glob_deeper_accepts_a_path_below_a_match():
    assert checker.names_glob(".local/share/claude", [".local/share/claude/versions"], True)
    assert checker.names_glob(".claude-*", [".claude-work/projects/x.jsonl"], True)
    assert not checker.names_glob(".local/share/claude", [".local/share/claude2/x"], True)
    assert not checker.names_glob(".local/share/claude", [".local/share"], True)


def test_base_remainders_forms():
    bases = [line(".hermes"), line("AppData/Local/hermes"), line(".hermes/profiles")]
    assert checker.base_remainders(".hermes*/models", bases) == ["models"]
    assert checker.base_remainders("AppData/Local/hermes*/a/b", bases) == ["a/b"]
    assert checker.base_remainders(".hermes/profiles/x", bases) == ["profiles/x", "x"]
    assert checker.base_remainders(".hermes", bases) == []
    assert checker.base_remainders(".other/models", bases) == []


def test_star_remainders_forms(tmp_path):
    text = "# home artifacts (agent|glob relative to home)\n"
    text += "alpha|Library/Application Support/Claude\nbeta|.claude\ngamma|AppData/Roaming/Codex\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    assert checker.star_remainders("*/Claude*/Cache", catalog) == [("alpha", "Cache")]
    assert checker.star_remainders("*/Roaming/Codex/web/Cache", catalog) == [("gamma", "web/Cache")]
    assert checker.star_remainders("*/Nothing/Cache", catalog) == []
    assert checker.star_remainders(".claude/Cache", catalog) == []


# --- Rule 2: credential-unflagged ------------------------------------------------------------


def test_credential_unflagged(tmp_path):
    assert findings(repository(tmp_path / "clean"), "credential-unflagged") == []
    catalog = CATALOG.replace("Library/Application Support/alpha/token.json\n", "")
    catalog = catalog.replace(".alpha/auth.json\n", "")
    root = repository(tmp_path / "drift", catalog=catalog)
    assert findings(root, "credential-unflagged") == [
        ("alpha", ".alpha/auth.json"),
        ("alpha", "Library/Application Support/alpha/token.json"),
    ]


def test_credential_sibling_on_one_platform_suffices(tmp_path):
    catalog = CATALOG.replace(
        "alpha|Library/Application Support/alpha\n",
        "alpha|Library/Application Support/alpha\nalpha|Library/Application Support/Alpha\n",
    )
    alpha = ALPHA.replace("on macOS.", "on macOS (`~/Library/Application Support/Alpha`).")
    root = repository(tmp_path, catalog=catalog, alpha=alpha)
    assert findings(root, "credential-unflagged") == []


def test_credential_named_in_secrets_is_not_reported(tmp_path):
    catalog = CATALOG.replace(".alpha/auth.json\n", "")
    secrets = "### Secrets\n\n`~/.alpha/auth.json` holds `token`; redact it.\n\n"
    alpha = ALPHA.replace("### Record schema", secrets + "### Record schema")
    root = repository(tmp_path / "named", catalog=catalog, alpha=alpha)
    assert findings(root, "credential-unflagged") == []
    other = secrets.replace(".alpha/auth.json", ".alpha/other.json")
    alpha = ALPHA.replace("### Record schema", other + "### Record schema")
    root = repository(tmp_path / "other", catalog=catalog, alpha=alpha)
    assert findings(root, "credential-unflagged") == [("alpha", ".alpha/auth.json")]


def test_secrets_store_forms():
    assert checker.secrets_store(".tabby/ee/db.sqlite", [".tabby/ee/db.sqlite"])
    assert checker.secrets_store(".tabby/ee/db.backup-.sqlite", [".tabby/ee/db.backup-*.sqlite"])
    assert not checker.secrets_store(".tabby/ee/db.sqlite", ["db.sqlite", "users.auth_token"])
    assert not checker.secrets_store(".tabby/ee/db.sqlite", [])


def test_is_path_span_forms():
    accepted = [
        ".alpha/auth.json",
        "C:\\Users\\x\\auth.json",
        "Library/Application Support/x",
        "auth.json",
        ".env",
        ".netrc",
        "secrets.yaml",
    ]
    rejected = [
        "https://example.com/a",
        "www.example.com",
        "KEY=value",
        "f(x)",
        "${HOME}/x",
        "<config home>",
        "key: value",
        "a, b",
        "api_key",
        "data.project",
        "token",
        "settings json",
        "",
    ]
    for span in accepted:
        assert checker.is_path_span(span), span
    for span in rejected:
        assert not checker.is_path_span(span), span


def test_is_file_span_forms():
    for span in (".alpha/auth.json", ".alpha/.env", "ide/*", "x/*.key", "auth.json"):
        assert checker.is_file_span(span), span
    for span in (".cline/data", ".config/amp/", "AppData\\Local\\x"):
        assert not checker.is_file_span(span), span


def test_base_form_forms():
    cases = {
        ".config/Claude": ("linux", {"config"}),
        ".local/share/zed": ("linux", {"data"}),
        ".local/state/x": ("linux", {"state"}),
        ".cache/x": ("linux", {"cache"}),
        ".var/app/dev.zed.Zed/config/zed": ("flatpak", {"config"}),
        ".var/app/dev.zed.Zed/data/zed": ("flatpak", {"data"}),
        "Library/Caches/x": ("macos", {"cache"}),
        "Library/Logs/x": ("macos", {"logs"}),
        "Library/Application Support/x": ("macos", {"home", "config", "data", "state"}),
        "AppData/Local/Temp/claude": ("windows", set()),
        "AppData/Local/Programs/muse": ("windows", set()),
        "AppData/Roaming/x": ("windows", {"home", "config", "data", "cache"}),
        ".hermes": ("unix", {"home"}),
        "Claude": ("", set()),
        "Documents/Claude": ("", set()),
    }
    for glob, form in cases.items():
        assert checker.base_form(glob) == form, glob


def test_sibling_bases_forms():
    entries = [
        line(".config/Claude"),
        line("Library/Application Support/Claude"),
        line("AppData/Roaming/claude"),
        line("Library/Logs/Claude"),
        line(".config/claude-two"),
        line("AppData/Local/Temp/claude"),
        line("Claude"),
        line("Library/Application Support/Unnamed/Claude"),
    ]
    named = [
        "Library/Application Support/Claude",
        "AppData/Roaming/claude",
        "Library/Logs/Claude",
        "AppData/Local/Temp/claude",
        "Claude",
    ]
    found = checker.sibling_bases(entries[0], entries, named)
    assert [entry.glob for entry in found] == [
        "Library/Application Support/Claude",
        "AppData/Roaming/claude",
    ]
    zed = [line(".config/zed"), line(".local/share/zed"), line(".var/app/z/config/zed")]
    found = checker.sibling_bases(zed[0], zed, [".local/share/zed", ".var/app/z/config/zed"])
    assert [entry.glob for entry in found] == [".var/app/z/config/zed"]
    homes = [line(".agent-zero"), line("agent-zero"), line("AppData/Local/agent-zero")]
    found = checker.sibling_bases(homes[0], homes, ["agent-zero", "AppData/Local/agent-zero"])
    assert [entry.glob for entry in found] == ["AppData/Local/agent-zero"]


def test_credential_paths_forms():
    entries = [line(".config/alpha"), line("Library/Application Support/alpha"), line(".alpha")]
    named = ["Library/Application Support/alpha"]
    assert checker.credential_paths(".config/alpha/token.json", entries, named) == [
        [".config/alpha/token.json"],
        ["Library/Application Support/alpha/token.json"],
    ]
    assert checker.credential_paths(".alpha/ide/*", entries, named) == [[".alpha/ide/x"]]
    assert checker.credential_paths(".alpha", entries, named) == []
    assert checker.credential_paths(".alpha/data", entries, named) == []
    assert checker.credential_paths("auth.json", entries, named) == []
    assert checker.credential_paths(".other/auth.json", entries, named) == []
    files = [line(".alpha.json"), line(".alpha")]
    assert checker.credential_paths(".alpha.json", files, []) == [[".alpha.json"]]


# --- Rule 3: secret-under-exclusion ----------------------------------------------------------


def test_secret_under_exclusion(tmp_path):
    assert findings(repository(tmp_path / "clean"), "secret-under-exclusion") == []
    catalog = CATALOG.replace(".alpha/auth.json\n", ".alpha/auth.json\n.alpha/cache/*.key\n")
    root = repository(tmp_path / "drift", catalog=catalog)
    assert findings(root, "secret-under-exclusion") == [("alpha", ".alpha/cache/*.key")]


def test_excluded_forms():
    exclusions = [".alpha/cache", "hermes-agent/[!.]*", "*/Claude*/Cache"]
    for path in (".alpha/cache", ".alpha/cache/x/y", "hermes-agent/src", "a/b/Claude/Cache/f"):
        assert checker.excluded(path, exclusions), path
    for path in (".alpha/cache2", ".alpha", "hermes-agent/.env", "Claude/Cache"):
        assert not checker.excluded(path, exclusions), path


def test_crossing_regex_forms():
    assert checker.crossing_regex("a/*/c").fullmatch("a/b/x/c")
    assert checker.crossing_regex("a?c").fullmatch("a/c")
    assert checker.crossing_regex("[!.]x").fullmatch("ax")
    assert not checker.crossing_regex("[!.]x").fullmatch(".x")
    assert checker.crossing_regex("a[b").fullmatch("a[b")
    assert checker.secret_samples(".a*/b*") == [".a/b", ".ax/bx"]


# --- Rule 4: exclusion-unreachable and secret-unreachable ------------------------------------


def test_exclusion_and_secret_unreachable(tmp_path):
    assert findings(repository(tmp_path / "clean"), "exclusion-unreachable") == []
    beta = BETA.replace(", and `models` there is excluded", "")
    catalog = CATALOG.replace("models\n#", "models\nnowhere/cache\n#")
    catalog = catalog.replace("token.json\n#", "token.json\nstray.key\n#")
    root = repository(tmp_path / "drift", beta=beta, catalog=catalog)
    assert findings(root, "exclusion-unreachable") == [("-", "models"), ("-", "nowhere/cache")]
    assert findings(root, "secret-unreachable") == [("-", "stray.key")]


def test_exclusion_below_a_project_entry_is_reachable(tmp_path):
    catalog = CATALOG.replace("alpha|.alpha\nproject|", "alpha|.alpha\nalpha|.alphaproj\nproject|")
    catalog = catalog.replace("models\n#", "models\n.alphaproj/big\n#")
    alpha = ALPHA.replace(
        "A project keeps `.alpha/`.", "A project keeps `.alpha/` and `.alphaproj/big`."
    )
    root = repository(tmp_path, catalog=catalog, alpha=alpha)
    assert findings(root, "exclusion-unreachable") == []
    assert findings(root, "line-unnamed") == []
    assert checker.load(root).glob_agents[".alphaproj/big"] == "alpha"


def test_exclusion_two_volume_documents_name_has_no_owner(tmp_path):
    catalog = CATALOG.replace("beta|*beta_data", "beta|*beta_data\nalpha|*alpha_data")
    alpha = ALPHA.replace("`cache` below `.alpha`.", "`cache` below `.alpha`, and `models`.")
    root = repository(tmp_path, catalog=catalog, alpha=alpha)
    found = [finding for finding in checker.run(root) if finding.kind == "exclusion-unreachable"]
    assert [(finding.agent, finding.detail) for finding in found] == [("-", "models")]
    assert "the documents of beta, alpha all name it" in found[0].message


def test_project_claim_forms(tmp_path):
    text = "# project artifacts (relative to each discovered project)\nproject|.git/worktrees\n"
    text += "pi|.pi\npi|.pi/approved-plan.md\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    assert checker.project_claim(".git/worktrees/*/index", catalog) == "project"
    assert checker.project_claim(".pi*/x", catalog) == "pi"
    assert checker.project_claim("tmp/playwright", catalog) == ""


def test_glob_agent_forms(tmp_path):
    text = (
        "# home artifacts (agent|glob relative to home)\nalpha|Library/Application Support/Claude\n"
    )
    text += "# project artifacts (relative to each discovered project)\nbeta|.beta\n"
    text += "# docker volumes (agent|volume name glob)\ngamma|*gamma_data\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    write(tmp_path / "research/gamma.md", "# gamma\n\n## Collection\n\n`models`\n")
    cases = {
        "Library/Application Support/Claude/Cache": "alpha",
        "*/Claude*/Cache": "alpha",
        "*/Other/Cache": "-",
        ".beta/cache": "beta",
        "models": "gamma",
        "nowhere": "-",
    }
    for glob, agent in cases.items():
        assert checker.glob_agent(glob, catalog, tmp_path) == agent, glob


# --- Rule 5: volume-glob-unnamed -------------------------------------------------------------


def test_volume_glob_unnamed(tmp_path):
    assert findings(repository(tmp_path / "clean"), "volume-glob-unnamed") == []
    beta = BETA.replace("#### Docker volume", "#### Container")
    beta = beta.replace("The `beta_data` volume", "The `beta_data` store")
    root = repository(tmp_path / "drift", beta=beta)
    found = [finding for finding in checker.run(root) if finding.kind == "volume-glob-unnamed"]
    assert [(finding.agent, finding.detail) for finding in found] == [("beta", "exclude models")]
    assert "looked in: none" in found[0].message
    assert findings(root, "exclusion-unreachable") == []


def test_is_volume_section_forms(tmp_path):
    text = """\
        # x

        ## Collection

        ### Per-user storage

        Nothing here.

        Later a volume.

        #### Docker volumes

        `models`.

        #### Container

        The named Volume holds
        the home.

        #### Bind mount

        The home is bind-mounted.
        """
    sections = sections_of(tmp_path, textwrap.dedent(text))
    verdicts = {key: checker.is_volume_section(section) for key, section in sections.items()}
    assert verdicts == {
        "": False,
        "Collection": False,
        "Collection/Per-user storage": False,
        "Collection/Per-user storage/Docker volumes": True,
        "Collection/Per-user storage/Container": True,
        "Collection/Per-user storage/Bind mount": False,
    }


# --- Rule 6: volume-twin-missing -------------------------------------------------------------


def test_volume_twin_missing(tmp_path):
    assert findings(repository(tmp_path / "clean"), "volume-twin-missing") == []
    root = repository(tmp_path / "drift", catalog=CATALOG.replace("\nmodels\n", "\n"))
    assert findings(root, "volume-twin-missing") == [("beta", ".beta/models")]


def test_volume_twin_as_a_suffix_of_the_home_exclusion(tmp_path):
    catalog = CATALOG.replace(".beta/models\n", ".beta/cache/models\n")
    assert findings(repository(tmp_path / "suffix", catalog=catalog), "volume-twin-missing") == []
    catalog = catalog.replace("\nmodels\n", "\nweights\n")
    root = repository(tmp_path / "none", catalog=catalog)
    assert findings(root, "volume-twin-missing") == [("beta", ".beta/cache/models")]


def test_volume_twin_forms():
    home = "AppData/Local/langflow/langflow/Cache/profile_pictures"
    remainders = checker.volume_home_remainders(home, "/app/langflow")
    assert remainders == ["langflow/Cache/profile_pictures", "Cache/profile_pictures"]
    assert checker.volume_twin(home, remainders, ["profile_pictures"])
    assert checker.volume_twin(home, remainders, ["Cache/profile_pictures"])
    assert checker.volume_twin(".ollama/models/blobs", ["models/blobs"], ["models"])
    assert not checker.volume_twin(home, remainders, ["pictures"])
    assert not checker.volume_twin(home, remainders, ["Cache/other"])
    assert not checker.volume_twin(home, remainders, [])


def test_is_volume_relative_forms(tmp_path):
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", CATALOG)).catalog
    assert checker.is_volume_relative("models", catalog)
    assert not checker.is_volume_relative(".beta/models", catalog)
    assert not checker.is_volume_relative("*/models", catalog)


def test_mount_path_forms():
    assert checker.mount_path(["Ollama: ollama; mounted at /root/.ollama"]) == "/root/.ollama"
    assert checker.mount_path(["Langflow, mounted at", "/app/langflow, the config dir"]) == (
        "/app/langflow"
    )
    assert checker.mount_path(["both are mounted at /a0/usr."]) == "/a0/usr"
    assert checker.mount_path(["(mounted at /data)"]) == "/data"
    assert checker.mount_path(["bind-mounts ~/.tabby at /data"]) == ""
    assert checker.mount_path([]) == ""


def test_volume_home_remainders_forms():
    assert checker.volume_home_remainders(".ollama/models", "/root/.ollama") == ["models"]
    assert checker.volume_home_remainders("agent-zero*/usr/x/y", "/a0/usr") == ["x/y"]
    assert checker.volume_home_remainders("a/langflow/langflow/x", "/app/langflow") == [
        "langflow/x",
        "x",
    ]
    assert checker.volume_home_remainders(".ollama", "/root/.ollama") == []
    assert checker.volume_home_remainders(".other/models", "/root/.ollama") == []
    assert checker.volume_home_remainders(".ollama/models", "") == []


# --- Rule 7: case-overlap --------------------------------------------------------------------


def test_case_overlap(tmp_path):
    assert findings(repository(tmp_path / "clean"), "case-overlap") == []
    catalog = CATALOG.replace("beta|.beta\n", "beta|.beta\nbeta|.config/Alpha\nbeta|.alpha*\n")
    root = repository(tmp_path / "drift", catalog=catalog)
    assert findings(root, "case-overlap") == [
        ("beta", ".config/alpha .config/Alpha"),
        ("beta", ".alpha .alpha*"),
    ]


def test_disjoint_prefix_globs_do_not_overlap(tmp_path):
    catalog = CATALOG.replace("beta|.beta\n", "beta|.beta\nbeta|.alpha-x\nbeta|.alpha*/argv.json\n")
    catalog = catalog.replace("alpha|.alpha\n", "alpha|.alpha\nalpha|.alpha/cody.json\n", 1)
    root = repository(tmp_path, catalog=catalog)
    assert findings(root, "case-overlap") == []


def test_common_path_forms():
    assert checker.common_path(".hermes*", ".hermes_x")
    assert checker.common_path(".alpha*", ".alpha")
    assert checker.common_path(".Alpha", ".alpha")
    assert not checker.common_path(".vscode", ".vscode-server*")
    assert not checker.common_path(".vscode*/argv.json", ".vscode/cody.json")
    assert not checker.common_path(".hermes*", ".hermes_x/sub")


def test_nested_forms():
    assert checker.nested(".gemini", ".gemini/antigravity-cli")
    assert checker.nested(".claude*", ".claude-x/projects")
    assert checker.nested(".config/alpha", ".config/alpha/sub")
    assert not checker.nested(".gemini/antigravity-cli", ".gemini")
    assert not checker.nested(".gemini", ".gemini")
    assert not checker.nested(".gemini", ".gemini2/x")


def test_case_overlap_forms():
    accepted = [
        (".config/PearAI", ".config/pearai"),
        (".hermes*", ".hermes_x"),
        (".Hermes*", ".hermes_x"),
        (".Alpha", ".alpha"),
    ]
    rejected = [
        (".vscode", ".vscode-server*"),
        (".vscode*/argv.json", ".vscode/cody.json"),
        (".vscode-server*/data/User", ".vscode/cody.json"),
        (".vscode-mock", ".vscode/cody.json"),
        ("AppData/Local/x", "AppData/LocalLow/y"),
        (".config/Code", ".config/Code - Insiders"),
        (".gemini", ".gemini/antigravity"),
        (".config/a", ".config/a"),
        (".alpha", ".beta"),
    ]
    for first, second in accepted:
        assert checker.case_overlap(first, second), (first, second)
    for first, second in rejected:
        assert not checker.case_overlap(first, second), (first, second)


# --- Rule 8: credential-wording --------------------------------------------------------------


def test_credential_wording(tmp_path):
    assert findings(repository(tmp_path / "clean"), "credential-wording") == []
    # The link text `a1b2c3d` is a citation, so the span before "may embed" is `state.db`.
    alpha = ALPHA.replace(
        "### Exclusions",
        "- `state.db` is collected unflagged; [`a1b2c3d`](https://x) settings may Embed keys\n"
        "  and `config.toml` can embed keys.\n\n### Exclusions",
    )
    root = repository(tmp_path / "drift", alpha=alpha)
    assert findings(root, "credential-wording") == [
        ("alpha", "collected unflagged `state.db`"),
        ("alpha", "may embed `state.db`"),
        ("alpha", "can embed `config.toml`"),
    ]


def test_credential_wording_without_a_span_before_it(tmp_path):
    alpha = ALPHA.replace(
        "- `~/.alpha/auth.json` holds", "Keys left unflagged.\n\n- `~/.alpha/auth.json` holds"
    )
    root = repository(tmp_path, alpha=alpha)
    assert findings(root, "credential-wording") == [("alpha", "left unflagged")]


def test_plain_spans_skip_link_texts(tmp_path):
    text = "# x\n\nA `one` and [`two`](https://x), `three\nwrapped` and [see `four`](y).\n"
    section = sections_of(tmp_path, text)[""]
    assert [text for text, _, _ in checker.plain_spans(section)] == [
        "one",
        "three wrapped",
        "four",
    ]


# --- Rule 9: presence ------------------------------------------------------------------------


def test_document_missing(tmp_path):
    assert findings(repository(tmp_path / "clean"), "document-missing") == []
    root = repository(tmp_path / "drift", beta="")
    assert findings(root, "document-missing") == [("beta", "research/beta.md")]
    root = repository(tmp_path / "shared", shared="")
    assert findings(root, "document-missing") == [("project", "research/shared.md")]


def test_catalog_missing(tmp_path):
    assert findings(repository(tmp_path / "clean"), "catalog-missing") == []
    root = repository(tmp_path / "drift")
    write(root / "research/delta.md", "# delta\n")
    write(root / "research/README.md", "# Research\n")
    assert findings(root, "catalog-missing") == [("delta", "delta.md")]


def test_agents_row_missing_and_orphan(tmp_path):
    clean = repository(tmp_path / "clean")
    assert findings(clean, "agents-row-missing") == []
    assert findings(clean, "agents-row-orphan") == []
    page = AGENTS_PAGE.replace("[`beta`](#beta)", "[`gamma`](#gamma)")
    page += "| Not An Agent | collected | x | y |\n"
    root = repository(tmp_path / "drift", page=page)
    assert findings(root, "agents-row-missing") == [("beta", "beta")]
    assert findings(root, "agents-row-orphan") == [
        ("gamma", "gamma"),
        ("-", "Not An Agent"),
    ]


def test_parsing_fact_under_collection(tmp_path):
    assert findings(repository(tmp_path / "clean"), "parsing-fact-under-collection") == []
    alpha = ALPHA.replace(
        "holds the sessions,",
        "holds the sessions (`session_id`, `message.content`, `cwd`, [`ab_cd`](x)) and writes "
        "`permission decision:` rows,",
    )
    root = repository(tmp_path / "drift", alpha=alpha)
    assert findings(root, "parsing-fact-under-collection") == [
        ("alpha", "session_id"),
        ("alpha", "message.content"),
        ("alpha", "permission decision:"),
    ]


def test_file_stem_span_is_no_parsing_fact(tmp_path):
    alpha = ALPHA.replace("holds the sessions,", "holds the sessions (`chatFeedback` events),")
    schema = "`session_id`, `project_dir` and `message.content`."
    events = schema + " Events go to `dev_data/0.2.0/chatFeedback.jsonl` with `chatFeedback`."
    root = repository(tmp_path / "named", alpha=alpha.replace(schema, events))
    assert findings(root, "parsing-fact-under-collection") == []
    events = schema + " Events go to `dev_data/0.2.0/events.jsonl` as `chatFeedback`."
    root = repository(tmp_path / "unnamed", alpha=alpha.replace(schema, events))
    assert findings(root, "parsing-fact-under-collection") == [("alpha", "chatFeedback")]


def test_path_names_forms():
    names = checker.path_names(["dev_data/0.2.0/chatFeedback.jsonl", "~/.alpha/", "session_id"])
    for name in ("dev_data", "0.2.0", "chatFeedback.jsonl", "chatFeedback", ".alpha"):
        assert name in names, name
    for name in ("session_id", "jsonl", "~", ""):
        assert name not in names, name


def test_is_row_text_forms():
    for span in ("permission decision:", "model: a -> b:", "session synced:"):
        assert checker.is_row_text(span), span
    for span in ("model:", "permission decision", "a: b"):
        assert not checker.is_row_text(span), span


def test_is_key_span_forms():
    for span in ("session_id", "data.project", "fullOutputPath", "message.content", "a.b_c"):
        assert checker.is_key_span(span), span
    for span in ("cwd", "mode", "state.db", ".env", "a/b", "API_KEY", "session-id", "x y"):
        assert not checker.is_key_span(span), span
