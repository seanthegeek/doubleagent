# pydantic-clai

The catalog agent `pydantic-clai` covers the Pydantic AI CLIs: `clai` v1, a chat CLI, and
CLAI 2.0 (`pydantic-clai2`), a terminal coding agent; both come from the Pydantic AI
repository. It came into the catalog from the agent framework sweep for
[issue #64](https://github.com/seanthegeek/doubleagent/issues/64); the Pydantic AI library
itself and Logfire (Pydantic's hosted tracing service), which got no catalog line, are covered
in [`research/frameworks.md`](frameworks.md). Its parser is in the backlog
([issue 66](https://github.com/seanthegeek/doubleagent/issues/66)).

## Source and evidence

[pydantic/pydantic-ai](https://github.com/pydantic/pydantic-ai), MIT, open source, commit
[`0ba01f92be3fc361f89970502cd6e7dd3611aec3`](https://github.com/pydantic/pydantic-ai/commit/0ba01f92be3fc361f89970502cd6e7dd3611aec3).
The repository ships `pydantic-ai(-slim)`, `pydantic-graph`,
`pydantic-evals`, the `clai` CLI, and under `src/` two newer packages:
`pydantic-ai-harness` and CLAI 2.0 (`pydantic-clai2`, commands `clai2` and
`pydantic-clai2`,
[pyproject.toml:62-65](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pyproject.toml#L62-L65);
`pydantic_clai2-0.54.0` is on PyPI and its wheel was fetched and read, not
installed). CLAI 2.0 is a terminal coding agent with a `coder` plugin on
by default
([README.md:3-5](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/README.md#L3-L5)).
All claims are from source.

CLAI 2.0 runs commands on the host: `LocalWorkspaceBackend` "runs commands
as plain host subprocesses — it isolates nothing"
([workspaces/local.py:1-5](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/pydantic_ai_slim/pydantic_ai/workspaces/local.py#L1-L5)),
and the harness ships a shell toolset
([shell/_toolset.py:50](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_ai_harness/pydantic_ai_harness/shell/_toolset.py#L50),
[365](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_ai_harness/pydantic_ai_harness/shell/_toolset.py#L365)).
`clai` v1 runs only provider-native tools
([_cli/\_\_init\_\_.py:60-62](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/pydantic_ai_slim/pydantic_ai/_cli/__init__.py#L60-L62)).

### Observed

- No real install: the `pydantic_clai2-0.54.0` wheel was fetched and read, not installed.

### Source reviews

- 2026-10-21: pydantic/pydantic-ai at
  [`62b7bf0`](https://github.com/pydantic/pydantic-ai/commit/62b7bf0aeac9a03066263469e650f40957d84cae),
  from source only: the state directory is now collected, with its workspace clones excluded
  and the workspace tokens flagged ([State directory](#state-directory),
  [Credentials](#credentials), [Exclusions](#exclusions)); the GitHub plugin's token sources are
  added to [Credentials](#credentials).
- 2026-10-19: [`62b7bf0`](https://github.com/pydantic/pydantic-ai/commit/62b7bf0aeac9a03066263469e650f40957d84cae) against
  [`0ba01f9`](https://github.com/pydantic/pydantic-ai/commit/0ba01f92be3fc361f89970502cd6e7dd3611aec3).
  Added CLAI 2.0's XDG state directory, which the catalog does not name, the harness step store
  and `metadata` keys of `sessions.db`, the saved model defaults in `config.db`, the workspaces
  plugin's scratch checkouts, and the absence of per-call approval and of session sync
  ([Per-user storage](#per-user-storage), [Credentials](#credentials), [Exclusions](#exclusions),
  [Where the project path is recorded](#where-the-project-path-is-recorded), [Parsing](#parsing)).

## Collection

### Per-user storage

The library has no default path under the home; message history is
returned to the caller (see [`research/frameworks.md`](frameworks.md)). The CLIs
write:

| Path (home-relative, all OSes) | Holds | Default or opt-in |
| --- | --- | --- |
| `.pydantic-ai/prompt-history.txt` | `clai` v1 prompt_toolkit input history ([_cli/\_\_init\_\_.py:52-58](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/pydantic_ai_slim/pydantic_ai/_cli/__init__.py#L52-L58), [460-463](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/pydantic_ai_slim/pydantic_ai/_cli/__init__.py#L460-L463)) | default for `clai` |
| `.config/pydantic-clai2/` (`$XDG_CONFIG_HOME` honoured; `~/.config` on macOS and Windows too) | the CLAI 2.0 folder ([settings_store.py:60-70](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/config/settings_store.py#L60-L70)) | default |
| `.config/pydantic-clai2/config.db` | settings (`settings` table), "never credentials or conversation messages" (same citation; [\_\_main\_\_.py:29](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/__main__.py#L29)); at [`62b7bf0`](https://github.com/pydantic/pydantic-ai/commit/62b7bf0aeac9a03066263469e650f40957d84cae) also the enabled plugins, `plugins(id, declaration)`, and the saved model defaults, tables `models`, `model_settings` and `model_chains` ([config/settings_store.py:83-102](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/config/settings_store.py#L83-L102)) | default |
| `.config/pydantic-clai2/sessions.db` | the sessions, table `conversations`, and the harness step store (schema in [Session store](#session-store)) ([_app.py:540](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/_app.py#L540), [conversations.py:37-58](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_ai_harness/pydantic_ai_harness/step_persistence/conversations.py#L37-L58), [154-156](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_ai_harness/pydantic_ai_harness/step_persistence/conversations.py#L154-L156)) | default |
| `.config/pydantic-clai2/input-history` | prompt history ([_app.py:789](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/_app.py#L789)) | default |
| `.config/pydantic-clai2/credentials-<account>.enc` (the keyring holds the Fernet key, service `pydantic-clai2`) or `credentials-<account>.json` (plaintext, no keyring), with `.lock` sidecars | provider logins, `api-keys`, Copilot ([credential_store.py:1-5](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/config/credential_store.py#L1-L5), [23-42](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/config/credential_store.py#L23-L42), [149-152](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/config/credential_store.py#L149-L152)) | on login |
| `.config/pydantic-clai2/mcp.json`, `mcp_logs/` | user MCP servers (headers "may be sensitive"), server logs ([mcp/_store.py:70-88](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/mcp/_store.py#L70-L88)) | on `/mcp install` |
| `.config/pydantic-clai2/logfire/logfire_credentials.json` | Logfire write token ([builtin_plugins/logfire.py:221-227](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/builtin_plugins/logfire.py#L221-L227)) | opt-in plugin |

#### Scratch directories

The workspaces plugin's clones under the state
directory and the worktrees it makes at `<repository>/.worktrees/<id>`
(or under a kind's `root`) are the agent's own checkouts; the clones are
under the home in the state directory, where the catalog excludes them
([Exclusions](#exclusions)), and the worktrees sit wherever the user's
repository is. Both are recorded in
`workspace.json` (`path`, `root`) and in a session's `metadata.workspace`
and `checkout.root`.

#### State directory

At [`62b7bf0`](https://github.com/pydantic/pydantic-ai/commit/62b7bf0aeac9a03066263469e650f40957d84cae) CLAI 2.0 keeps state
that is not a preference in `$XDG_STATE_HOME/pydantic-clai2` when that
variable is absolute, else `.local/state/pydantic-clai2`, on every OS
([config/settings_store.py:60-69](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/config/settings_store.py#L60-L69)).
The catalog line `pydantic-clai|.local/state/pydantic-clai2` collects it. Its contents:

| Path under `.local/state/pydantic-clai2/` | Holds | Default or opt-in |
| --- | --- | --- |
| `orchestrator.json` | the web UI orchestrator's own conversation, the agent that creates workspaces and gives their agents tasks; never saved in `sessions.db` ([builtin_plugins/web/orchestrator.py:1-7](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/web/orchestrator.py#L1-L7), [:49](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/web/orchestrator.py#L49), [:137-138](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/web/orchestrator.py#L137-L138)) | web UI with the workspaces plugin |
| `workspaces/registry/<id>/workspace.json` | a workspace record: `id`, `kind`, `repo`, `ref`, `branch`, checkout `path` and `root`, `status`, `created`, `teardown`, `model`, and while it runs its CLAI's `pid`, `port`, `started` ([builtin_plugins/workspaces/registry.py:40-79](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/workspaces/registry.py#L40-L79)) | workspaces plugin |
| `workspaces/registry/<id>/token`, `session.log` | the access token of the workspace's `clai2 --web` process and that process's output ([builtin_plugins/workspaces/sessions.py:118-127](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/workspaces/sessions.py#L118-L127)) | while the workspace runs |
| `workspaces/clones/<repo slug>-<10 hex>/` | git clones the plugin makes of remote repositories, with the workspaces' worktrees in `.worktrees/<id>` inside unless the workspace kind names a root ([builtin_plugins/workspaces/manager.py:49-52](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/workspaces/manager.py#L49-L52), [backend.py:79-98](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/workspaces/backend.py#L79-L98)) | workspaces plugin, remote repository |
| `aoe/` | which CLAI conversation an Agent of Empires (a tmux session manager) session holds ([builtin_plugins/aoe.py:1-7](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/aoe.py#L1-L7), [:67-69](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/aoe.py#L67-L69)) | opt-in plugin |

Destroying a workspace stops its CLAI, removes the checkout when its
teardown says so, and deletes its registry folder with the token and the
log
([builtin_plugins/workspaces/manager.py:139-149](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/workspaces/manager.py#L139-L149),
[registry.py:117-119](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/workspaces/registry.py#L117-L119));
the session the workspace's CLAI saved stays in `sessions.db`.

#### Session store

`sessions.db` and `input-history` are beside `config.db`, opened at [`62b7bf0`](https://github.com/pydantic/pydantic-ai/commit/62b7bf0aeac9a03066263469e650f40957d84cae) from
[runtime/process.py:48](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/runtime/process.py#L48)
and
[runtime/session_runtime.py:560](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/runtime/session_runtime.py#L560).
`sessions.db` has `conversations(id, revision, metadata, messages, search_text, updated_at)`, the
same columns at both commits
([step_persistence/conversations.py:174-177](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_ai_harness/pydantic_ai_harness/step_persistence/conversations.py#L174-L177));
`metadata` gained `checkout` (`repository`, `name`, `root`, `branch` of the
git checkout, kept so a session can be grouped after its folder is gone)
and `archived_at`
([:38-48](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_ai_harness/pydantic_ai_harness/step_persistence/conversations.py#L38-L48),
[:52-75](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_ai_harness/pydantic_ai_harness/step_persistence/conversations.py#L52-L75)).
The same file now holds the harness step store, which CLAI opens for
every run with up to eight message snapshots per run
([runtime/_session.py:74-76](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/runtime/_session.py#L74-L76)):
`runs`, `events(kind, step_index, timestamp, tool_call_id, tool_name,
error, metadata)`, `snapshots(messages)` with
`snapshot_messages`, and `tool_effects(tool_call_id, tool_name, status,
started_at, ended_at, effect_summary)`
([step_persistence/_store.py:951-1040](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_ai_harness/pydantic_ai_harness/step_persistence/_store.py#L951-L1040)).
A `sessions.repl/` directory beside `sessions.db` keeps each session's
`run_code` REPL state
([runtime/repl_state.py:1-10](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/runtime/repl_state.py#L1-L10)).
Deleting a session removes its `conversations` row and, in the same
transaction, its runs, events, snapshots and tool effects
([conversations.py:357-377](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_ai_harness/pydantic_ai_harness/step_persistence/conversations.py#L357-L377)),
then its REPL file
([runtime/sessions.py:345-350](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/runtime/sessions.py#L345-L350));
the prompt history files keep its prompts. Archiving deletes nothing.

#### Other agents' sessions

CLAI 2.0's `/resume` reads Claude Code
sessions (`~/.claude/projects/*/*.jsonl`) and Codex rollouts
(`~/.codex/sessions/**`) and copies them into `sessions.db`
([imported_sessions.py:1-5](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/runtime/imported_sessions.py#L1-L5),
[claude_code_sessions.py:41-43](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/runtime/claude_code_sessions.py#L41-L43),
[codex_sessions.py:35-37](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/runtime/codex_sessions.py#L35-L37)).
A Claude Code or Codex session can therefore appear in CLAI's store as
well as in its own; see [`research/claude-code.md`](claude-code.md) and
[`research/codex-cli.md`](codex-cli.md). Both are already in the catalog.

#### Docker

None.

### Credentials

- `.config/pydantic-clai2/credentials-*` (the `.enc` and `.json` forms and
  their `.lock` sidecars): flagged. Without a keyring the `.json` form is
  plaintext; with one, the `.enc` form needs the keyring's Fernet key.
- `.config/pydantic-clai2/logfire/logfire_credentials.json`: flagged.
- `.config/pydantic-clai2/mcp.json` and a project's `.clai/mcp_servers.json` (the same server
  model, [Project-local files](#project-local-files)) give stdio servers `env` and HTTP servers
  `headers`, which can embed tokens; both are flagged.
- `.local/state/pydantic-clai2/workspaces/registry/<id>/token`
  is the access token of a running workspace's local `clai2 --web`
  server, plaintext, owner-only ([State directory](#state-directory)):
  flagged, by the glob
  `.local/state/pydantic-clai2/workspaces/registry/*/token`. Slack
  sign-in tokens go to the credential store under the account
  `slack-oauth`
  ([slack_app.py:15](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/slack_app.py#L15)),
  so a `credentials-slack-oauth.*` file is already flagged.
- The GitHub token of the built-in `github` plugin, which gives the agent GitHub's hosted MCP
  tools, comes from the GitHub CLI's sign-in or from a key saved with `/keys`, by default under
  the name `GITHUB_TOKEN`
  ([builtin_plugins/github.py:1-9](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/github.py#L1-L9),
  [:34](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/github.py#L34)).
  The CLI's token is read with `gh auth token` on every run and stays in `gh`'s own store, the
  OS keyring, so CLAI keeps no copy of it
  ([gh_cli.py:1-5](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/gh_cli.py#L1-L5),
  [:58-74](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/gh_cli.py#L58-L74)).
  Saved keys are one bundle in the credential store under the account `api-keys`
  ([config/api_keys.py:114-121](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/config/api_keys.py#L114-L121)),
  so a `credentials-api-keys.*` file is already flagged. The plugin's settings hold only
  `login` (`gh` or `key`), the key's name and the MCP endpoint `url`
  ([builtin_plugins/github.py:48-54](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/github.py#L48-L54)).

### Exclusions

`.local/state/pydantic-clai2/workspaces/clones`, the full git clones of remote repositories
the workspaces plugin makes, with the workspaces' worktrees inside them
([State directory](#state-directory)): excluded, since they are copies of repositories that
exist elsewhere; what a workspace's agent did is recorded in its session.

CLAI 2.0 puts git worktrees at
`<repo>/.worktrees/<name>`
([worktrees.py:46](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/runtime/worktrees.py#L46));
`.worktrees` is not a project catalog entry, and it is too generic a name
to exclude.

### Project-local files

`.clai/settings.json` and `.clai/mcp_servers.json`
([project_settings.py:10](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/config/project_settings.py#L10),
[mcp/_store.py:25](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/mcp/_store.py#L25)),
collected through the `pydantic-clai|.clai` project catalog entry.

### Where the project path is recorded

`sessions.db`, `conversations.metadata`, JSON key `workspace`. It is
recorded only inside SQLite, so `discover_projects` and `Find-Projects`
cannot read it; a project's `.clai` directory is collected when another
agent's state or a `-p` option names the project. The analyzer is the
place to read it.

`metadata.checkout.root` holds the git
checkout's top-level folder beside `workspace`, also inside SQLite. A
workspace's `workspace.json` (plain JSON, keys `path` and `root`, and
`repo` when that is a local path) names the agent's own checkouts, which
are scratch directories rather than projects
([Scratch directories](#scratch-directories)).

## Parsing

`pydantic-clai` is `backlog`: its parser is
[issue 66](https://github.com/seanthegeek/doubleagent/issues/66), and nothing it writes is parsed
yet. A parser would read CLAI 2.0's `.config/pydantic-clai2/sessions.db`: the `conversations`
table (`metadata` with `workspace`, `model`, `title`, `checkout` and `archived_at`; `messages`)
and the harness step store in the same file (`runs`, `events`, `snapshots`, `tool_effects`)
([Session store](#session-store)), with the prompt histories `input-history` and
`clai` v1's `.pydantic-ai/prompt-history.txt`, which keep a deleted session's prompts, and the
saved model defaults in `config.db`. The orchestrator's `orchestrator.json` and the workspace
records sit in the state directory, which is collected
([State directory](#state-directory)); a session imported from Claude Code or Codex repeats
that agent's own store ([Other agents' sessions](#other-agents-sessions)). `input-history` and
`prompt-history.txt` are line files and `orchestrator.json` a whole-file JSON document.

CLAI 2.0 asks for no per-call tool approval: "CLAI runs tools without approval"
([builtin_plugins/google_workspace.py:45](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/google_workspace.py#L45)),
and `ApprovalRequired` and `DeferredTool` occur nowhere in `src/pydantic_clai2` at
[`62b7bf0`](https://github.com/pydantic/pydantic-ai/commit/62b7bf0aeac9a03066263469e650f40957d84cae) (the `approv` matches are plugin approvals, OAuth sign-in prompts and a prompt
instruction). A project's plugins stay disabled until approved with `/plugins enable`
([config/project_settings.py:27](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/config/project_settings.py#L27)),
and enabled plugins are kept in `config.db` ([Per-user storage](#per-user-storage)). Destroying a
workspace needs the user's `Destroy <id>` answer to an `ask_user_question` call, which stays in
the orchestrator's messages
([builtin_plugins/web/workspace_tools.py:80-102](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/web/workspace_tools.py#L80-L102)).
The model is recorded per conversation in `conversations.metadata.model`, per response in the
pydantic-ai messages, and per workspace in its record's `model`.

No session is synced. The `observability` plugin, on by default, sends run traces to Pydantic
Logfire when a token is present (`send_to_logfire` `if-token-present`), without prompt and
response content unless `include_content` is turned on
([builtin_plugins/logfire.py:1-14](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/logfire.py#L1-L14),
[:70-73](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/builtin_plugins/logfire.py#L70-L73)).
The Slack integration is an MCP sign-in, not a relay
([slack_app.py:1-15](https://github.com/pydantic/pydantic-ai/blob/62b7bf0aeac9a03066263469e650f40957d84cae/src/pydantic_clai2/pydantic_clai2/slack_app.py#L1-L15)).

## Confidence

- High: everything stated from source at the cited commits.
- Not determined: whether the `credentials-*.lock` files hold anything (the docstring says the
  SQLite lock file holds no secrets,
  [credential_store.py:80](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/config/credential_store.py#L80));
  the full `messages` encoding in `sessions.db` (media externalised to a `SqliteMediaStore`),
  which needs a record-schema study under [Parsing](#parsing) before a parser is written.
