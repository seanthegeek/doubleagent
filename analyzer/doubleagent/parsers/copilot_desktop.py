"""Parse the app store of GitHub Copilot Desktop (the Tauri app, dpkg package `github`).

The app runs its sessions through a bundled Copilot CLI that writes them into
`.copilot/session-state/<id>/` in the CLI's own format, which the `copilot-cli` parser reads
(`workspace.yaml` `client_name` `github/autopilot` marks them). This parser reads the app's own
store only, none of it a secondary source, and yields `system` rows:

- `.copilot/data.db`, SQLite in WAL mode, every column possibly NULL and every time ISO 8601
  UTC; a missing table gives no rows. Tables, in row order: `accounts` (`login`, `host`, `kind`,
  `is_default`, `access_token` as present or not, `created_at`), `settings`
  (`storage_location`, `instructions`, `github_access_token`, `github_token_type`; at the file's
  mtime), `model_providers`, `projects`, `worktrees`, `workspaces`, `sessions` (`id`, `title`,
  `session_type`, `model`, `auto_approve`, `permission_mode`, `remote_control_enabled`,
  `archived_at`, `forked_from_session_id`, `created_at`; the project through
  `workspace_session_aliases` and `workspaces.project_id`), `activity_items`,
  `session_checkpoints`, `my_copilot_scheduled_tasks`, `my_copilot_scheduled_task_triggers`,
  `my_copilot_scheduled_task_invocations` (at `claimed_at`), `session_automations` (at
  `updated_at`) and `session_deletion_intents`. A token is never written, only whether one is
  present.
- `.copilot/repos/<repo>/.git/refs/copilot/checkpoints/<session id>/<seq>/<event id>`, the
  loose git ref the app writes into its clone of a project per checkpoint: one row at the file's
  mtime from the path groups and the content as read, stripped.

The model is `sessions.model` per session; `data.db` keeps no per-switch history. Validated
against the shipped app `github` 1.1.29 linux-arm64 and a real install; the scheduling tables
and `session_deletion_intents` are fixture only. See `research/copilot-desktop.md`.
"""

from __future__ import annotations

import re
import sqlite3
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import open_copy, sqlite_problem, table_names
from ..timeutil import to_utc
from .base import Options, Parser, compact_json, error_rows, file_bytes, file_mtime

DATABASE_PATTERN = re.compile(r"^\.copilot/data\.db$")
REF_PATTERN = re.compile(
    r"^\.copilot/repos/[^/]+/\.git/refs/copilot/checkpoints/"
    r"([0-9a-f-]{36})/(\d+)/([0-9a-f-]{36})$"
)


def _str(value) -> str:
    if value is None:
        return ""
    return value if isinstance(value, str) else compact_json(value)


def _dash(value) -> str:
    return _str(value) or "-"


def _bool(value) -> str:
    return "false" if value in (None, "") else "true"


class CopilotDesktopParser(Parser):
    """Parse Copilot Desktop's `data.db` and its checkpoint refs."""

    agent = "copilot-desktop"
    name = "copilot-desktop"

    def wants(self, artifact: Artifact) -> bool:
        """Return whether the artifact is `data.db` or a checkpoint ref."""
        return bool(DATABASE_PATTERN.match(artifact.rel) or REF_PATTERN.match(artifact.rel))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of `data.db`, or the one row of a checkpoint ref."""
        ref = REF_PATTERN.match(artifact.rel)
        if ref:
            yield self._ref_row(artifact, ref)
            return
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield self._row(artifact, "parser: " + problem)
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                rows = list(self._rows(artifact, connection))
        except sqlite3.DatabaseError as error:
            yield self._row(artifact, "parser: SQLite error: %s" % error)
            return
        yield from rows

    def _ref_row(self, artifact: Artifact, ref: re.Match) -> Row:
        session_id, sequence, event_id = ref.groups()
        errors: list = []
        data = file_bytes(artifact.disk_path, errors)
        if data is None:
            return next(error_rows(self, artifact, errors, session_id))
        return self._row(
            artifact,
            "checkpoint ref %d: commit %s event %s (file mtime)"
            % (int(sequence), data.decode("utf-8", "replace").strip(), event_id),
            file_mtime(artifact),
            session_id=session_id,
        )

    def _row(self, artifact: Artifact, text: str, timestamp=None, **columns) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.text = compact(text)
        row.timestamp_utc = to_utc(timestamp) if timestamp not in (None, "") else ""
        for field, value in columns.items():
            setattr(row, field, value or "")
        return row

    @staticmethod
    def _select(connection: sqlite3.Connection, tables: set, table: str, order: str) -> list[dict]:
        if table not in tables:
            return []
        sql = "select rowid as _rowid, * from %s order by %s, rowid" % (table, order)
        return [dict(row) for row in connection.execute(sql)]

    def _rows(self, artifact: Artifact, connection: sqlite3.Connection) -> Iterator[Row]:
        tables = table_names(connection)

        def select(table: str, order: str = "created_at") -> list[dict]:
            return self._select(connection, tables, table, order)

        projects = {_str(project.get("id")): project for project in select("projects")}
        workspaces = select("workspaces")
        workspace_project = {
            _str(workspace.get("id")): _str(workspace.get("project_id")) for workspace in workspaces
        }
        aliases = {
            _str(alias.get("session_id")): _str(alias.get("workspace_id"))
            for alias in select("workspace_session_aliases")
        }

        def project_path(project_id: str) -> str:
            return _str(projects.get(project_id, {}).get("main_repo_path"))

        for account in select("accounts"):
            yield self._row(
                artifact,
                "account: %s host=%s kind=%s default=%s token_present=%s"
                % (
                    _str(account.get("login")),
                    _str(account.get("host")),
                    _str(account.get("kind")),
                    _str(account.get("is_default")),
                    _bool(account.get("access_token")),
                ),
                account.get("created_at"),
            )
        mtime = file_mtime(artifact)
        for settings_row in select("settings", "id"):
            yield self._row(
                artifact,
                "desktop settings: storage_location=%s instructions=%s"
                " github_token_present=%s github_token_type=%s (from %s, file mtime)"
                % (
                    _str(settings_row.get("storage_location")),
                    _dash(settings_row.get("instructions")),
                    _bool(settings_row.get("github_access_token")),
                    _dash(settings_row.get("github_token_type")),
                    artifact.rel,
                ),
                mtime,
            )
        for provider in select("model_providers"):
            yield self._row(
                artifact,
                "model provider: %s type=%s"
                % (_str(provider.get("name")), _str(provider.get("type"))),
                provider.get("created_at"),
            )
        for project in projects.values():
            text = "project added: %s kind=%s path=%s" % (
                _str(project.get("name")),
                _str(project.get("container_kind")),
                _str(project.get("main_repo_path")),
            )
            if project.get("github_owner") is not None or project.get("github_repo") is not None:
                text += " github=%s/%s" % (
                    _str(project.get("github_owner")),
                    _str(project.get("github_repo")),
                )
            text += " default_branch=%s" % _str(project.get("default_branch"))
            yield self._row(
                artifact,
                text,
                project.get("created_at"),
                project_path=_str(project.get("main_repo_path")),
            )
        for workspace in select("worktrees"):
            yield self._row(
                artifact,
                "worktree: %s branch=%s base=%s"
                % (
                    _str(workspace.get("path")),
                    _str(workspace.get("branch")),
                    _dash(workspace.get("base_branch")),
                ),
                workspace.get("created_at"),
                project_path=project_path(_str(workspace.get("project_id"))),
            )
        for workspace in workspaces:
            text = "workspace: %s type=%s branch=%s" % (
                _str(workspace.get("name")),
                _str(workspace.get("workspace_type")),
                _str(workspace.get("branch")),
            )
            for prefix, label in (("source_pr", "source_pr"), ("created_pr", "created_pr")):
                repo, number = (
                    workspace.get(prefix + "_repo_full_name"),
                    workspace.get(prefix + "_number"),
                )
                if repo is not None or number is not None:
                    text += " %s=%s#%s" % (label, _str(repo), _str(number))
            yield self._row(
                artifact,
                text,
                workspace.get("created_at"),
                project_path=project_path(_str(workspace.get("project_id"))),
            )
        yield from self._session_rows(artifact, select, aliases, workspace_project, project_path)
        yield from self._schedule_rows(artifact, select)

    def _session_rows(self, artifact, select, aliases, workspace_project, project_path):
        for session in select("sessions"):
            session_id = _str(session.get("id"))
            columns = {
                "session_id": session_id,
                "model": _str(session.get("model")),
                "project_path": project_path(
                    workspace_project.get(aliases.get(session_id, ""), "")
                ),
            }
            yield self._row(
                artifact,
                "desktop session: %s type=%s model=%s auto_approve=%s permission_mode=%s"
                " remote_control=%s archived=%s"
                % (
                    _dash(session.get("title")),
                    _str(session.get("session_type")),
                    _dash(session.get("model")),
                    _str(session.get("auto_approve")),
                    _dash(session.get("permission_mode")),
                    _str(session.get("remote_control_enabled")),
                    _bool(session.get("archived_at")),
                ),
                session.get("created_at"),
                **columns,
            )
            if session.get("forked_from_session_id") not in (None, ""):
                yield self._row(
                    artifact,
                    "forked from session %s" % _str(session.get("forked_from_session_id")),
                    session.get("created_at"),
                    **columns,
                )
        for activity in select("activity_items"):
            yield self._row(
                artifact,
                "activity %s: %s"
                % (_str(activity.get("activity_type")), _str(activity.get("preview"))),
                activity.get("created_at"),
                session_id=_str(activity.get("session_id")),
            )
        for checkpoint in select("session_checkpoints"):
            yield self._row(
                artifact,
                "checkpoint commit %s" % _str(checkpoint.get("commit_sha")),
                checkpoint.get("created_at"),
                session_id=_str(checkpoint.get("session_id")),
            )

    def _schedule_rows(self, artifact, select):
        for task in select("my_copilot_scheduled_tasks"):
            yield self._row(
                artifact,
                "scheduled task: %s enabled=%s mode=%s agent=%s prompt: %s"
                % (
                    _str(task.get("title")),
                    _str(task.get("enabled")),
                    _str(task.get("execution_mode")),
                    _str(task.get("background_agent_type")),
                    _str(task.get("prompt")),
                ),
                task.get("created_at"),
            )
        for trigger in select("my_copilot_scheduled_task_triggers"):
            yield self._row(
                artifact,
                "scheduled task trigger: task=%s kind=%s enabled=%s next_run_at=%s config: %s"
                % (
                    _str(trigger.get("task_id")),
                    _str(trigger.get("kind")),
                    _str(trigger.get("enabled")),
                    _dash(trigger.get("next_run_at")),
                    _str(trigger.get("config_json")),
                ),
                trigger.get("created_at"),
            )
        for invocation in select("my_copilot_scheduled_task_invocations", "claimed_at"):
            yield self._row(
                artifact,
                "scheduled task run: %s status=%s source=%s scheduled_for=%s prompt: %s"
                % (
                    _str(invocation.get("task_title")),
                    _str(invocation.get("status")),
                    _str(invocation.get("source")),
                    _dash(invocation.get("scheduled_for")),
                    _str(invocation.get("prompt_snapshot")),
                ),
                invocation.get("claimed_at"),
                session_id=_str(invocation.get("parent_session_id")),
            )
        for automation in select("session_automations", "updated_at"):
            yield self._row(
                artifact,
                "session automation: %s interval=%s every_minutes=%s next_run_at=%s prompt: %s"
                % (
                    _dash(automation.get("name")),
                    _str(automation.get("interval")),
                    _dash(automation.get("every_minutes")),
                    _dash(automation.get("next_run_at")),
                    _str(automation.get("prompt")),
                ),
                automation.get("updated_at"),
                session_id=_str(automation.get("session_id")),
            )
        for intent in select("session_deletion_intents"):
            yield self._row(
                artifact,
                "session deletion requested",
                intent.get("created_at"),
                session_id=_str(intent.get("session_id")),
            )
