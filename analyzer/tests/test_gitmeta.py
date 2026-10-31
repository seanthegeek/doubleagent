"""Tests of `gitmeta`: the remotes and HEAD of a collected project's repository."""

import os
import shutil
import tempfile
import unittest
from pathlib import Path

from doubleagent import gitmeta
from doubleagent.inputs import Artifact, Home
from fixtures import PROJECT_GIT_CONFIG, PROJECT_GIT_HEAD, PROJECT_GIT_REMOTES


class ParseRemotesTests(unittest.TestCase):
    """`parse_remotes` reads a repository config as git does."""

    def test_fixture_config(self):
        self.assertEqual(gitmeta.parse_remotes(PROJECT_GIT_CONFIG), PROJECT_GIT_REMOTES)

    def test_pushurl_is_what_the_file_says(self):
        # git falls back to `url` for pushes; the file has no pushurl, so none.
        got = gitmeta.parse_remotes('[remote "origin"]\n\turl = https://h/r.git\n')
        self.assertEqual(got, [{"name": "origin", "urls": ["https://h/r.git"], "pushurls": []}])

    def test_case_comments_quotes_and_continuation(self):
        text = (
            "# a comment\n"
            "; another\n"
            '[Remote "Fork"]\n'
            '  URL = "https://h/a;b#c.git"  # trailing comment\n'
            "  pushUrl = https://h/\\\n"
            "push.git\n"
            '[remote "second"]\n'
            "\turl = https://h/one.git\n"
            "\tpushurl = https://h/push-a.git\n"
            "\turl = https://h/two.git\n"
            "\tpushurl = https://h/push-b.git\n"
            "\turl =\n"
            "[remote.legacy]\n"
            "\turl = /srv/mirror.git\n"
            '[branch "main"]\n'
            "\turl = not-a-remote\n"
        )
        self.assertEqual(
            gitmeta.parse_remotes(text),
            [
                {
                    "name": "Fork",
                    "urls": ["https://h/a;b#c.git"],
                    "pushurls": ["https://h/push.git"],
                },
                {"name": "legacy", "urls": ["/srv/mirror.git"], "pushurls": []},
                # several url and pushurl lines: every value, in file order;
                # an empty value is skipped
                {
                    "name": "second",
                    "urls": ["https://h/one.git", "https://h/two.git"],
                    "pushurls": ["https://h/push-a.git", "https://h/push-b.git"],
                },
            ],
        )

    def test_comment_backslash_does_not_continue(self):
        # A backslash that ends a full-line or a trailing comment is not a
        # continuation, so the header after it keeps its own remote; one that
        # ends a value still is.
        text = (
            '[remote "origin"]\n'
            "\turl = https://h/origin.git  # cloned from C:\\git\\\n"
            '[remote "backup"]\n'
            "\turl = https://h/backup.git\n"
            "# mirrors live under D:\\git\\\n"
            '[remote "mirror"]\n'
            "\turl = https://h/\\\n"
            "mirror.git ; comment with a backslash \\\n"
            '[remote "last"]\n'
            "\turl = https://h/last.git\n"
        )
        self.assertEqual(
            gitmeta.parse_remotes(text),
            [
                {"name": "backup", "urls": ["https://h/backup.git"], "pushurls": []},
                {"name": "last", "urls": ["https://h/last.git"], "pushurls": []},
                {"name": "mirror", "urls": ["https://h/mirror.git"], "pushurls": []},
                {"name": "origin", "urls": ["https://h/origin.git"], "pushurls": []},
            ],
        )

    def test_quoted_and_escaped_whitespace_is_kept(self):
        # Only bare whitespace around a value is trimmed, as git trims it; a
        # quoted or escaped space is part of the path. Bare whitespace before a
        # comment still goes.
        text = (
            '[remote "a"]\n'
            '\turl =   "/srv/mirror.git "  \n'
            '[remote "b"]\n'
            '\turl = " /srv/lead.git"\t; note\n'
            '[remote "c"]\n'
            "\turl = /srv/plain.git\\t \n"
            '[remote "d"]\n'
            '\turl = "/srv/in ner.git"\n'
        )
        self.assertEqual(
            [(remote["name"], remote["urls"]) for remote in gitmeta.parse_remotes(text)],
            [
                ("a", ["/srv/mirror.git "]),
                ("b", [" /srv/lead.git"]),
                ("c", ["/srv/plain.git\t"]),
                ("d", ["/srv/in ner.git"]),
            ],
        )

    def test_unterminated_quote_is_dropped_and_reported(self):
        # A config cut inside a quoted value: git rejects the line, so the
        # partial text is not a destination; the pair is left out, the valid
        # remotes stay, and the line is reported.
        text = (
            '[remote "origin"]\n'
            "\turl = https://h/origin.git\n"
            '[remote "cut"]\n'
            "\tpushurl = https://h/cut-push.git\n"
            '\turl = "https://example.com/team/part'
        )
        errors: list[str] = []
        self.assertEqual(
            gitmeta.parse_remotes(text, errors),
            [
                {"name": "cut", "urls": [], "pushurls": ["https://h/cut-push.git"]},
                {"name": "origin", "urls": ["https://h/origin.git"], "pushurls": []},
            ],
        )
        self.assertEqual(errors, ["line 5: unterminated quoted value"])
        # A quote closed on a continued line is fine.
        self.assertEqual(
            gitmeta.parse_remotes('[remote "a"]\n\turl = "https://h/\\\na.git"\n', errors),
            [{"name": "a", "urls": ["https://h/a.git"], "pushurls": []}],
        )
        self.assertEqual(len(errors), 1)

    def test_unsupported_escape_is_dropped_and_reported(self):
        # git defines \\, \", \n, \t and \b and a backslash before a line
        # end; any other escape makes it reject the line, so the backslash
        # is never dropped to invent a destination.
        text = (
            '[remote "origin"]\n'
            "\turl = https://h/origin.git\n"
            '[remote "bad"]\n'
            "\turl = https://host/team\\q.git\n"
            "\tpushurl = https://h/bad-push.git\n"
            '[remote "spaced"]\n'
            "\turl = /srv/with\\ space.git\n"
        )
        errors: list[str] = []
        self.assertEqual(
            gitmeta.parse_remotes(text, errors),
            [
                {"name": "bad", "urls": [], "pushurls": ["https://h/bad-push.git"]},
                {"name": "origin", "urls": ["https://h/origin.git"], "pushurls": []},
            ],
        )
        self.assertEqual(
            errors, ["line 4: unsupported escape \\q", "line 7: unsupported escape \\ "]
        )

    def test_dotted_header_subsection_is_lower_cased(self):
        # The deprecated [section.subsection] form lower-cases its subsection
        # (git), so [remote.Origin] and [remote "origin"] are one remote; a
        # quoted subsection keeps its case.
        text = (
            "[remote.Origin]\n"
            "\turl = https://h/origin.git\n"
            '[remote "origin"]\n'
            "\tpushurl = https://h/origin-push.git\n"
            '[remote "Origin"]\n'
            "\turl = https://h/capital.git\n"
        )
        self.assertEqual(
            gitmeta.parse_remotes(text),
            [
                {"name": "Origin", "urls": ["https://h/capital.git"], "pushurls": []},
                {
                    "name": "origin",
                    "urls": ["https://h/origin.git"],
                    "pushurls": ["https://h/origin-push.git"],
                },
            ],
        )

    def test_bom_crlf_and_unicode_line_separators(self):
        # git accepts a UTF-8 byte-order mark at the start of a config and
        # ends a line at LF or CRLF only; a LINE SEPARATOR or NEXT LINE inside a quoted
        # path is part of the path.
        text = (
            '﻿[remote "origin"]\r\n'
            "\turl = https://h/origin.git\r\n"
            '[remote "odd"]\n'
            '\turl = "/srv/team\N{LINE SEPARATOR}archive.git"\n'
            '\tpushurl = "/srv/team\N{NEXT LINE}archive.git"\n'
        )
        self.assertEqual(
            gitmeta.parse_remotes(text),
            [
                {
                    "name": "odd",
                    "urls": ["/srv/team\N{LINE SEPARATOR}archive.git"],
                    "pushurls": ["/srv/team\N{NEXT LINE}archive.git"],
                },
                {"name": "origin", "urls": ["https://h/origin.git"], "pushurls": []},
            ],
        )

    def test_padded_value_is_trimmed_in_one_pass(self):
        # A value padded with many spaces, as untrusted evidence may be, is
        # trimmed without a pop(0) per space.
        pad = " " * 200_000
        text = '[remote "p"]\n\turl =%s"https://h/p.git "%s\n' % (pad, pad)
        self.assertEqual(
            gitmeta.parse_remotes(text),
            [{"name": "p", "urls": ["https://h/p.git "], "pushurls": []}],
        )

    def test_pair_on_the_header_line(self):
        # git allows a variable after its section header on the same line; a
        # comment there is still a comment.
        text = (
            '[remote "origin"] url = https://h/origin.git\n'
            '[remote "note"] # just a comment\n'
            "\turl = https://h/note.git\n"
            '[remote "two"]\turl = https://h/\\\n'
            "two.git ; pushurl = not-a-key\n"
        )
        self.assertEqual(
            gitmeta.parse_remotes(text),
            [
                {"name": "note", "urls": ["https://h/note.git"], "pushurls": []},
                {"name": "origin", "urls": ["https://h/origin.git"], "pushurls": []},
                {"name": "two", "urls": ["https://h/two.git"], "pushurls": []},
            ],
        )

    def test_malformed_lines_are_skipped(self):
        text = (
            '[remote "origin"]\n'
            "\tthis line has no equals sign\n"
            "\turl = https://h/origin.git\n"
            '[remote "broken"\n'  # unterminated header: its keys belong to no remote
            "\turl = https://h/broken.git\n"
            "= no key\n"
            '[remote "upstream"]\n'
            "\turl = https://h/upstream.git\n"
            "\x00\xff garbage\n"
        )
        self.assertEqual(
            gitmeta.parse_remotes(text),
            [
                {"name": "origin", "urls": ["https://h/origin.git"], "pushurls": []},
                {"name": "upstream", "urls": ["https://h/upstream.git"], "pushurls": []},
            ],
        )

    def test_empty_and_remote_without_url(self):
        self.assertEqual(gitmeta.parse_remotes(""), [])
        self.assertEqual(gitmeta.parse_remotes('[remote "x"]\n\tfetch = +refs/*\n'), [])
        # A section with only a push URL is listed.
        self.assertEqual(
            gitmeta.parse_remotes('[remote "p"]\n\tpushurl = https://h/p.git\n'),
            [{"name": "p", "urls": [], "pushurls": ["https://h/p.git"]}],
        )


class ParseHeadTests(unittest.TestCase):
    """`parse_head` reads a HEAD file: a branch, a detached commit or another ref."""

    def test_unicode_in_a_branch_name(self):
        # A ref may hold a LINE SEPARATOR or a trailing NO-BREAK SPACE; the line ends at the
        # line feed and only ASCII whitespace is trimmed.
        self.assertEqual(
            gitmeta.parse_head("ref: refs/heads/team\N{LINE SEPARATOR}release\N{NO-BREAK SPACE}\n"),
            "team\N{LINE SEPARATOR}release\N{NO-BREAK SPACE}",
        )
        self.assertEqual(gitmeta.parse_head("ref: refs/heads/main\r\n"), "main")
        self.assertEqual(gitmeta.parse_head("\n  ref: refs/heads/second \t\n"), "second")

    def test_branch(self):
        self.assertEqual(gitmeta.parse_head(PROJECT_GIT_HEAD), "main")
        self.assertEqual(gitmeta.parse_head("ref: refs/heads/feature/x"), "feature/x")

    def test_detached(self):
        sha = "0123456789abcdef0123456789abcdef01234567"
        self.assertEqual(gitmeta.parse_head(sha + "\n"), sha)

    def test_other_ref_and_empty(self):
        self.assertEqual(
            gitmeta.parse_head("ref: refs/remotes/origin/main\n"), "refs/remotes/origin/main"
        )
        self.assertEqual(gitmeta.parse_head(""), "")
        self.assertEqual(gitmeta.parse_head("\n\n"), "")


class ProjectGitTests(unittest.TestCase):
    """`project_git` finds a collected project's repository files and reads them."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-gitmeta-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def artifact(self, home: Home, rel: str, text: str | None, agent: str = "project") -> Artifact:
        disk = self.tmp / home.original.strip("/") / rel
        disk.parent.mkdir(parents=True, exist_ok=True)
        if text is not None:
            disk.write_text(text, encoding="utf-8")
        return Artifact(disk, home.original + "/" + rel, rel, agent, home)

    def aside(self, home: Home, rel: str, text: str) -> Artifact:
        """An artifact of `home` at `rel` whose copy is written apart from the others.

        An artifact of `home` at `rel` whose copy is written apart from the others, so two spellings
        of one name never share a file on a case-insensitive volume.
        """
        disk = (
            self.tmp
            / "aside"
            / str(len(os.listdir(self.tmp / "aside")) if (self.tmp / "aside").is_dir() else "0")
        )
        disk.mkdir(parents=True, exist_ok=True)
        disk = disk / rel.replace("/", "_")
        disk.write_text(text, encoding="utf-8")
        return Artifact(disk, home.original + "/" + rel, rel, "project", home)

    def test_project_git(self):
        project = Home(self.tmp / "srv/proj", "/srv/proj", "alice", project=True)
        bare = Home(self.tmp / "srv/bare", "/srv/bare", "alice", project=True)
        home = Home(self.tmp / "home/alice", "/home/alice", "alice")
        arts = [
            self.artifact(project, ".git/config", PROJECT_GIT_CONFIG),
            self.artifact(project, ".git/HEAD", PROJECT_GIT_HEAD),
            self.artifact(bare, "AGENTS.md", "# x\n"),
            # A home's .git is not a project's: never read.
            self.artifact(home, ".git/config", PROJECT_GIT_CONFIG),
        ]
        self.assertEqual(
            gitmeta.project_git(arts),
            {"/srv/proj": {"remotes": PROJECT_GIT_REMOTES, "head": "main"}},
        )

    def test_unterminated_quote_is_a_problem(self):
        project = Home(self.tmp / "srv/proj", "/srv/proj", "alice", project=True)
        config = self.artifact(
            project, ".git/config", PROJECT_GIT_CONFIG + '[remote "cut"]\n\turl = "x'
        )
        problems: list[str] = []
        got = gitmeta.project_git([config], problems)
        self.assertEqual(got["/srv/proj"]["remotes"], PROJECT_GIT_REMOTES)
        self.assertEqual(
            problems,
            [
                "/srv/proj/.git/config: line %d: unterminated quoted value"
                % (PROJECT_GIT_CONFIG.count("\n") + 2)
            ],
        )

    def test_case_preserved_names_match_and_exact_names_win(self):
        # The PowerShell collector matches the catalog case-insensitively and
        # keeps the on-disk spelling, on Windows and on a macOS volume alike,
        # so .GIT/CONFIG and .git/head are the repository's files when no
        # exact spelling was collected; where a case-sensitive volume holds
        # both spellings, git's own wins whatever order the rows come in.
        win = Home(self.tmp / "c/proj", "C:\\proj", "alice", project=True)
        mac = Home(self.tmp / "Users/alice/proj", "/Users/alice/proj", "alice", project=True)
        both = Home(self.tmp / "srv/proj", "/srv/proj", "alice", project=True)
        arts = [
            self.artifact(win, ".GIT/CONFIG", PROJECT_GIT_CONFIG),
            self.artifact(win, ".git/head", PROJECT_GIT_HEAD),
            self.artifact(mac, ".GIT/config", PROJECT_GIT_CONFIG),
            self.artifact(mac, ".git/HEAD", PROJECT_GIT_HEAD),
            # Case-insensitive spelling first, exact second, and the reverse.
            # The variants' copies live apart on disk, since on a
            # case-insensitive volume (this Mac) the two spellings would be
            # one file and the later write would replace the earlier.
            self.aside(both, ".git/head", "ref: refs/heads/other\n"),
            self.artifact(both, ".git/HEAD", PROJECT_GIT_HEAD),
            self.artifact(both, ".git/config", PROJECT_GIT_CONFIG),
            self.aside(both, ".GIT/CONFIG", '[remote "x"]\n\turl = https://h/x.git\n'),
        ]
        self.assertEqual(
            gitmeta.project_git(arts),
            {
                "C:\\proj": {"remotes": PROJECT_GIT_REMOTES, "head": "main"},
                "/Users/alice/proj": {"remotes": PROJECT_GIT_REMOTES, "head": "main"},
                "/srv/proj": {"remotes": PROJECT_GIT_REMOTES, "head": "main"},
            },
        )

    def test_missing_unreadable_and_symlinked_files(self):
        project = Home(self.tmp / "srv/proj", "/srv/proj", "alice", project=True)
        # A collected row whose file is gone reads as empty.
        missing = self.artifact(project, ".git/HEAD", None)
        self.assertEqual(gitmeta.project_git([missing]), {"/srv/proj": {"remotes": [], "head": ""}})
        # A symlink is never followed.
        target = self.tmp / "elsewhere"
        target.write_text(PROJECT_GIT_CONFIG, encoding="utf-8")
        link = self.tmp / "srv/proj/.git/config"
        try:
            os.symlink(target, link)
        except (OSError, NotImplementedError):
            self.skipTest("symlinks not available")
        config = Artifact(link, "/srv/proj/.git/config", ".git/config", "project", project)
        self.assertEqual(gitmeta.read_text(link), ("", "not a regular file"))
        problems: list[str] = []
        self.assertEqual(gitmeta.project_git([config], problems)["/srv/proj"]["remotes"], [])
        self.assertEqual(problems, ["/srv/proj/.git/config: not a regular file"])

    def test_unreadable_file_is_a_problem(self):
        # A permission or I/O error is reported, never read as "no remotes";
        # the other project is still read.
        if os.name == "nt" or not hasattr(os, "geteuid") or os.geteuid() == 0:
            self.skipTest("chmod does not deny access as root or on Windows")
        project = Home(self.tmp / "srv/proj", "/srv/proj", "alice", project=True)
        other = Home(self.tmp / "srv/other", "/srv/other", "alice", project=True)
        config = self.artifact(project, ".git/config", PROJECT_GIT_CONFIG)
        head = self.artifact(other, ".git/HEAD", PROJECT_GIT_HEAD)
        os.chmod(config.disk_path, 0)
        try:
            problems: list[str] = []
            got = gitmeta.project_git([config, head, config], problems)
            # Without a list to report to, the read failure is still not a result.
            alone = gitmeta.project_git([config])
        finally:
            os.chmod(config.disk_path, 0o600)
        self.assertEqual(
            got,
            {
                "/srv/proj": {"remotes": [], "head": ""},
                "/srv/other": {"remotes": [], "head": "main"},
            },
        )
        self.assertEqual(len(problems), 1, problems)
        self.assertTrue(problems[0].startswith("/srv/proj/.git/config: "), problems[0])
        self.assertEqual(alone["/srv/proj"]["remotes"], [])

    def test_whole_file_is_read(self):
        # No size cap: a remote past the first MiB is read in full.
        project = Home(self.tmp / "srv/proj", "/srv/proj", "alice", project=True)
        text = (
            "# " + "x" * (1 << 20) + '\n[remote "far"]\n\turl = https://host/team/repository.git\n'
        )
        config = self.artifact(project, ".git/config", text)
        self.assertEqual(
            gitmeta.project_git([config])["/srv/proj"]["remotes"],
            [{"name": "far", "urls": ["https://host/team/repository.git"], "pushurls": []}],
        )


def test_header_and_key_separators_are_space_or_tab_only():
    # git allows only space and tab around a section name, between it and its
    # subsection and between a key and `=`; another space makes a bad line.
    for space in ("\u00a0", "\u2028", "\u0085", "\u3000"):
        for text in (
            '[%sremote "origin"]\n\turl = https://h/r.git\n' % space,
            '[remote%s"origin"]\n\turl = https://h/r.git\n' % space,
            '[remote "origin"%s]\n\turl = https://h/r.git\n' % space,
            '[remote "origin"]\n\turl%s= https://h/r.git\n' % space,
        ):
            assert gitmeta.parse_remotes(text) == [], repr(text)
    for space in (" ", "\t", " \t"):
        text = '[%sremote%s"origin"%s]\n\turl%s= https://h/r.git\n' % ((space,) * 4)
        assert gitmeta.parse_remotes(text) == [
            {"name": "origin", "urls": ["https://h/r.git"], "pushurls": []}
        ], repr(text)


if __name__ == "__main__":
    unittest.main()
