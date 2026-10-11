"""Claude Desktop local agent sessions (Cowork, the Code tab, scheduled tasks).

Closed source. The shapes come from the app bundle 2.31226.1
(`Contents/Resources/app.asar`, macOS), read without running it, with the
top-level keys of `scheduled-tasks.json` and `git-worktrees.json` observed
on an install of the same version; see `research/claude-desktop.md`. Paths
below are relative to the app-data directory, one of the catalog bases
`Library/Application Support/Claude[-3p]`, `.config/Claude[-3p]`,
`AppData/Roaming/Claude[-3p]` or `AppData/Local/Claude-3p`; `<acct>` and
`<org>` are account and organization UUIDs (or their first eight hex
digits). Files:

* `local-agent-mode-sessions/<acct>/<org>/[agent/]<session dir>/.claude/
  projects/<slug>/<cliSessionId>.jsonl`, the subagent files under
  `<cliSessionId>/subagents/`, the same under
  `claude-code-sessions/<acct>/<org>/<session dir>/`, and
  `claude-code-sessions/<acct>/<org>/imported-staging/<cliSessionId>.jsonl`:
  Claude Code JSONL, read by `ClaudeCodeParser._parse_session`, so the
  field names in `claude_code.py` apply. The rows keep the JSONL
  `sessionId`. `project_path` is taken from the session record's
  `userSelectedFolders[0]` when the record is found and has one, because
  the JSONL `cwd` is the guest path `/sessions/<name>` when the CLI runs
  in the desktop's VM; otherwise it is the JSONL `cwd` as claude-code sets
  it. The record is `<org>/local_<uuid>.json` or
  `<org>/agent/local_<uuid>.json`, found from the session dir name
  `local_<uuid>` or its eight-hex short form; staged imports have no
  session dir and keep the JSONL `cwd`.
* `local-agent-mode-sessions/<acct>/<org>/[agent/]<session dir>/audit.jsonl`:
  one Agent SDK message per line plus `_audit_timestamp` (ISO 8601) and
  `_audit_hmac`. Read: `type`, `subtype`, `timestamp`, `_audit_timestamp`,
  `session_id`, `cwd`, `model`, `client_platform`, `isSynthetic`,
  `message.model`, `message.content` blocks (`text`, `tool_use` with `id`,
  `name`, `input`; `tool_result` with `tool_use_id`, `content`,
  `is_error`; `thinking`, `redacted_thinking`), `tool_name`, `tool_input`,
  `decision`, `granted`, `source`, `scheduled_task_id`, `matched_ops`,
  `num_turns`, `is_error`. Permission events (`system` subtypes
  `permission_request`, `permission_response`, `permission_auto_approved`,
  `permission_auto_denied`) and `result` lines are always `system` rows.
  The `user`, `assistant` and `system` `init` lines duplicate the
  transcript and are read only when the session dir holds no
  `.claude/projects/*/*.jsonl` (the full fallback): desktop prompts
  (`client_platform` `desktop_app`) are `user` rows, `isSynthetic`
  notifications and other user text are `context:` system rows. The
  session id is the record's `cliSessionId`, else the line's `session_id`;
  `project_path` is the record's `userSelectedFolders[0]`, else its `cwd`.
* `local-agent-mode-sessions/<acct>/<org>/[agent/]local_<uuid>.json` (Cowork)
  and `claude-code-sessions/<acct>/<org>/local_<uuid>.json` (Code tab):
  `sessionId`, `cliSessionId`, `sessionType`, `title`, `createdAt`,
  `model`, `userSelectedFolders`, `originCwd`, `cwd`, `branch` (Code tab),
  `initialMessage`, `error`, `errorCategory`, `errorAt`. One `system` row
  at `createdAt`; `initialMessage` becomes a `user` row only when neither
  a transcript nor an audit log for the session is in the input.
* `.../scheduled-tasks.json`: `scheduledTasks[]` with `id`,
  `cronExpression` or `fireAt`, `enabled`, `createdAt`, `cwd`,
  `userSelectedFolders`.
* `git-worktrees.json`: `worktrees{}` values with `path`, `branch`,
  `sourceBranch`, `baseRepo`, `leasedBy` (the desktop session id, kept in
  the text, not as `session_id`), `createdAt`.

Every `createdAt`, `errorAt` and `fireAt` is epoch milliseconds, which
`to_utc` takes as milliseconds because the value is above 1e11.

Not parsed: the claude.ai LevelDB stores, the logs, `spaces.json`,
memory files, `rpm/` and caches.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from pathlib import Path

from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from .base import Options, compact_json, iter_jsonl, text_of
from .claude_code import ClaudeCodeParser, result_text, tool_summary

BASE = (
    r"^(?:Library/Application Support/Claude(?:-3p)?"
    r"|\.config/Claude(?:-3p)?"
    r"|AppData/Roaming/Claude(?:-3p)?"
    r"|AppData/Local/Claude-3p)/"
)
_TREES = r"(?:local-agent-mode-sessions|claude-code-sessions)/[^/]+/[^/]+/"
# Group `sess` is the session dir, relative to the home.
TRANSCRIPT_RX = re.compile(
    r"(?P<sess>"
    + BASE
    + _TREES
    + r"(?:agent/)?[^/]+)/\.claude/projects/[^/]+/"
    + r"(?:[^/]+\.jsonl|[^/]+/subagents/(?:[^/]+/)*agent-[^/]+\.jsonl)$"
)
STAGED_RX = re.compile(BASE + r"claude-code-sessions/[^/]+/[^/]+/imported-staging/[^/]+\.jsonl$")
AUDIT_RX = re.compile(
    r"(?P<sess>" + BASE + r"local-agent-mode-sessions/[^/]+/[^/]+/(?:agent/)?[^/]+)/audit\.jsonl$"
)
RECORD_RX = re.compile(BASE + r"(?P<tree>" + _TREES + r")(?:agent/)?local_[0-9a-f-]{36}\.json$")
SCHED_RX = re.compile(BASE + _TREES + r"scheduled-tasks\.json$")
WORKTREE_RX = re.compile(BASE + r"git-worktrees\.json$")

LONG_RX = re.compile(r"^local_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
SHORT_RX = re.compile(r"^[0-9a-f]{8}$")


def _bool(v) -> str:
    """`true`/`false` as in the JSON, the value as text otherwise."""
    if v is None:
        return ""
    return json.dumps(v) if isinstance(v, bool) else str(v)


def _str(v) -> str:
    return "" if v is None else str(v)


def _read_json(path: Path):
    """The parsed file, or None when it cannot be read or parsed."""
    try:
        return json.loads(path.read_bytes().decode("utf-8", errors="replace"))
    except (OSError, ValueError):
        return None


def _plain_dir(d: Path) -> bool:
    return not d.is_symlink() and d.is_dir()


def _plain_file(f: Path) -> bool:
    return not f.is_symlink() and f.is_file()


def _org_dir(sess: Path) -> Path:
    return sess.parent.parent if sess.parent.name == "agent" else sess.parent


def _find_record(sess: Path) -> dict | None:
    """The session record of a session dir named `local_<uuid>` or with the
    UUID's first eight hex digits, from `<org>/` or `<org>/agent/`; never
    through a symlink."""
    org = _org_dir(sess)
    name = sess.name
    if not (LONG_RX.match(name) or SHORT_RX.match(name)):
        return None
    if org.is_symlink() or sess.parent.is_symlink():
        return None
    for d in (org, org / "agent"):
        if not _plain_dir(d):
            continue
        if LONG_RX.match(name):
            candidates = [d / (name + ".json")]
        else:
            try:
                candidates = sorted(d.glob("local_%s-*.json" % name))
            except OSError:
                candidates = []
        for f in candidates:
            if not LONG_RX.match(f.name[: -len(".json")]) or not _plain_file(f):
                continue
            rec = _read_json(f)
            if isinstance(rec, dict):
                return rec
    return None


def _jsonl_files(d: Path, name: str = "*.jsonl") -> list[Path]:
    """`d/*/<name>` regular files, through no symlink."""
    if not _plain_dir(d):
        return []
    out = []
    try:
        for slug in sorted(d.iterdir()):
            if _plain_dir(slug):
                out.extend(f for f in sorted(slug.glob(name)) if _plain_file(f))
    except OSError:
        return out
    return out


def _has_transcript(sess: Path) -> bool:
    claude = sess / ".claude"
    return _plain_dir(sess) and _plain_dir(claude) and bool(_jsonl_files(claude / "projects"))


def _folder(rec: dict | None) -> str:
    if not rec:
        return ""
    folders = rec.get("userSelectedFolders")
    if isinstance(folders, list) and folders and folders[0]:
        return str(folders[0])
    return ""


class ClaudeDesktopParser(ClaudeCodeParser):
    agent = "claude-desktop"
    name = "claude-desktop"

    def wants(self, artifact: Artifact) -> bool:
        rel = artifact.rel
        return any(
            rx.match(rel)
            for rx in (TRANSCRIPT_RX, STAGED_RX, AUDIT_RX, RECORD_RX, SCHED_RX, WORKTREE_RX)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        rel = artifact.rel
        m = TRANSCRIPT_RX.match(rel)
        if m:
            yield from self._transcript(artifact, opts, self._sess(artifact, m.group("sess")))
        elif STAGED_RX.match(rel):
            yield from self._transcript(artifact, opts, None)
        elif AUDIT_RX.match(rel):
            m = AUDIT_RX.match(rel)
            assert m is not None
            yield from self._audit(artifact, opts, self._sess(artifact, m.group("sess")))
        elif RECORD_RX.match(rel):
            yield from self._record(artifact)
        elif SCHED_RX.match(rel):
            yield from self._scheduled(artifact)
        elif WORKTREE_RX.match(rel):
            yield from self._worktrees(artifact)

    @staticmethod
    def _sess(artifact: Artifact, sess_rel: str) -> Path:
        return artifact.home.disk_path / sess_rel

    def _system(self, artifact: Artifact, text: str, line: int = 0) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.source_line = line
        row.text = compact(text)
        return row

    # -- transcripts -----------------------------------------------------------

    def _transcript(self, artifact: Artifact, opts: Options, sess: Path | None) -> Iterator[Row]:
        folder = _folder(_find_record(sess)) if sess is not None else ""
        for row in self._parse_session(artifact, opts):
            if folder:
                row.project_path = folder
            yield row

    # -- audit.jsonl -----------------------------------------------------------

    def _audit(self, artifact: Artifact, opts: Options, sess: Path) -> Iterator[Row]:
        rec = _find_record(sess)
        full = not _has_transcript(sess)
        project = _folder(rec) or (_str(rec.get("cwd")) if rec else "")
        cli_id = _str(rec.get("cliSessionId")) if rec else ""
        errors: list = []
        last_sid = cli_id
        for n, line in iter_jsonl(artifact.disk_path, errors):
            sid = cli_id or _str(line.get("session_id"))
            last_sid = sid or last_sid
            ts = to_utc(line.get("timestamp")) or to_utc(line.get("_audit_timestamp"))

            def row(turn_type: str, text: str, n=n, ts=ts, sid=sid) -> Row:
                r = self.base_row(artifact)
                r.source_line = n
                r.timestamp_utc = ts
                r.session_id = sid
                r.project_path = project
                r.turn_type = turn_type
                r.text = compact(text)
                return r

            ltype = line.get("type")
            subtype = line.get("subtype")
            if ltype == "system" and subtype in (
                "permission_request",
                "permission_response",
                "permission_auto_approved",
                "permission_auto_denied",
            ):
                tool = _str(line.get("tool_name"))
                if subtype == "permission_request":
                    inp = line.get("tool_input")
                    text = "permission request: %s %s" % (
                        tool,
                        "" if inp is None else tool_summary(tool, inp),
                    )
                elif subtype == "permission_response":
                    text = "permission response: %s %s granted=%s" % (
                        tool,
                        _str(line.get("decision")),
                        _bool(line.get("granted")),
                    )
                else:
                    word = "approved" if subtype == "permission_auto_approved" else "denied"
                    why = line.get("source") or line.get("scheduled_task_id")
                    if not why and line.get("matched_ops") is not None:
                        why = compact_json(line.get("matched_ops"))
                    text = "permission auto-%s: %s" % (word, tool)
                    if why:
                        text += " (%s)" % why
                r = row("system", text)
                r.tool_name = tool
                yield r
            elif ltype == "result":
                yield row(
                    "system",
                    "result: %s turns=%s error=%s"
                    % (
                        _str(line.get("subtype")),
                        _str(line.get("num_turns")),
                        _bool(line.get("is_error")),
                    ),
                )
            elif not full:
                continue
            elif ltype == "system" and subtype == "init":
                parts = ["%s=%s" % (k, line.get(k)) for k in ("cwd", "model") if line.get(k)]
                yield row("system", "init: " + " ".join(parts) if parts else "init")
            elif ltype == "user":
                yield from self._audit_user(line, row)
            elif ltype == "assistant":
                yield from self._audit_assistant(line, row, opts)
        if errors:
            r = self._system(
                artifact,
                "parser: %d unparseable line(s), first at line %d" % (len(errors), errors[0][0]),
                errors[0][0],
            )
            r.session_id = last_sid
            r.project_path = project
            yield r

    @staticmethod
    def _audit_user(line: dict, row) -> Iterator[Row]:
        content = (line.get("message") or {}).get("content")
        blocks = content if isinstance(content, list) else [{"type": "text", "text": content}]
        typed = line.get("client_platform") == "desktop_app" and not line.get("isSynthetic")
        for b in blocks:
            if not isinstance(b, dict):
                continue
            btype = b.get("type")
            if btype == "tool_result":
                text = result_text(b.get("content"))
                if b.get("is_error"):
                    text = "[error] " + text
                r = row("tool_result", text)
                r.tool_use_id = _str(b.get("tool_use_id"))
                yield r
            elif btype in ("text", "image"):
                text = text_of([b])
                if not text:
                    continue
                if typed:
                    yield row("user", text)
                elif line.get("isSynthetic"):
                    yield row("system", "context: notification: " + text)
                else:
                    yield row("system", "context: " + text)

    @staticmethod
    def _audit_assistant(line: dict, row, opts: Options) -> Iterator[Row]:
        msg = line.get("message") or {}
        model = _str(msg.get("model"))
        content = msg.get("content")
        blocks = content if isinstance(content, list) else [{"type": "text", "text": content}]
        for b in blocks:
            if not isinstance(b, dict):
                continue
            btype = b.get("type")
            if btype == "tool_use":
                name = _str(b.get("name"))
                r = row("tool_use", tool_summary(name, b.get("input")))
                r.tool_name = name
                r.tool_use_id = _str(b.get("id"))
            elif btype == "text" and b.get("text"):
                r = row("assistant", _str(b.get("text")))
            elif btype == "thinking" and opts.include_thinking and b.get("thinking"):
                r = row("thinking", _str(b.get("thinking")))
            elif btype == "redacted_thinking" and opts.include_thinking:
                r = row("thinking", "[redacted]")
            else:
                continue
            r.model = model
            yield r

    # -- session records -------------------------------------------------------

    def _record(self, artifact: Artifact) -> Iterator[Row]:
        m = RECORD_RX.match(artifact.rel)
        assert m is not None
        code = m.group("tree").startswith("claude-code-sessions/")
        rec = _read_json(artifact.disk_path)
        if not isinstance(rec, dict):
            yield self._system(artifact, "parser: session record is not a JSON object", 1)
            return
        cli_id = _str(rec.get("cliSessionId"))
        project = _folder(rec) or _str(rec.get("originCwd")) or _str(rec.get("cwd"))
        created = to_utc(rec.get("createdAt"))
        model = _str(rec.get("model"))
        title = _str(rec.get("title"))

        def row(turn_type: str, text: str, ts: str = created) -> Row:
            r = self.base_row(artifact)
            r.source_line = 1
            r.timestamp_utc = ts
            r.session_id = cli_id
            r.project_path = project
            r.git_branch = _str(rec.get("branch")) if code else ""
            r.model = model
            r.turn_type = turn_type
            r.text = compact(text)
            return r

        if code:
            yield row("system", 'desktop code session: "%s"' % title)
        else:
            yield row("system", 'desktop session: %s "%s"' % (_str(rec.get("sessionType")), title))
        initial = _str(rec.get("initialMessage"))
        if initial and not self._session_collected(artifact, rec):
            yield row("user", initial)
        if rec.get("error"):
            cat = _str(rec.get("errorCategory"))
            err = _str(rec.get("error"))
            yield row(
                "system",
                "session error: %s" % ("%s: %s" % (cat, err) if cat else err),
                to_utc(rec.get("errorAt")),
            )

    @staticmethod
    def _session_collected(artifact: Artifact, rec: dict) -> bool:
        """Whether a transcript or audit log for the record's session is in
        the input: the session dir (full or short name, beside the record or
        in its org dir), the org's `imported-staging/`, or for the Code tab
        `~/.claude/projects/*/`."""
        rdir = artifact.disk_path.parent
        org = rdir.parent if rdir.name == "agent" else rdir
        sid = _str(rec.get("sessionId")) or artifact.disk_path.stem
        names = [sid]
        if LONG_RX.match(sid):
            names.append(sid[len("local_") : len("local_") + 8])
        for d in dict.fromkeys((rdir, org)):
            for name in names:
                sess = d / name
                if not _plain_dir(sess):
                    continue
                if _plain_file(sess / "audit.jsonl") or _has_transcript(sess):
                    return True
        cli_id = _str(rec.get("cliSessionId"))
        if cli_id:
            staging = org / "imported-staging"
            if _plain_dir(staging) and _plain_file(staging / (cli_id + ".jsonl")):
                return True
            claude = artifact.home.disk_path / ".claude"
            if _plain_dir(claude) and _jsonl_files(claude / "projects", cli_id + ".jsonl"):
                return True
        return False

    # -- scheduled tasks and worktrees -----------------------------------------

    def _scheduled(self, artifact: Artifact) -> Iterator[Row]:
        doc = _read_json(artifact.disk_path)
        tasks = doc.get("scheduledTasks") if isinstance(doc, dict) else None
        if not isinstance(tasks, list):
            yield self._system(artifact, "parser: unreadable scheduled-tasks.json", 1)
            return
        for i, task in enumerate(tasks, 1):
            if not isinstance(task, dict):
                continue
            when = task.get("cronExpression") or to_utc(task.get("fireAt"))
            r = self._system(
                artifact,
                "scheduled task: %s %s enabled=%s"
                % (_str(task.get("id")), _str(when), _bool(task.get("enabled"))),
                i,
            )
            r.timestamp_utc = to_utc(task.get("createdAt"))
            r.project_path = _str(task.get("cwd")) or _folder(task)
            yield r

    def _worktrees(self, artifact: Artifact) -> Iterator[Row]:
        doc = _read_json(artifact.disk_path)
        trees = doc.get("worktrees") if isinstance(doc, dict) else None
        if not isinstance(trees, dict):
            yield self._system(artifact, "parser: unreadable git-worktrees.json", 1)
            return
        for i, wt in enumerate(trees.values(), 1):
            if not isinstance(wt, dict):
                continue
            text = "worktree created: %s branch=%s from=%s" % (
                _str(wt.get("path")),
                _str(wt.get("branch")),
                _str(wt.get("sourceBranch")),
            )
            if wt.get("leasedBy"):
                text += " leased by %s" % wt.get("leasedBy")
            r = self._system(artifact, text, i)
            r.timestamp_utc = to_utc(wt.get("createdAt"))
            r.project_path = _str(wt.get("baseRepo"))
            r.git_branch = _str(wt.get("branch"))
            yield r
