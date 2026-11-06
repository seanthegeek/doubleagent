# Relocation warnings

An agent can keep its files somewhere other than its default folder when an
environment variable says so: `CLAUDE_CONFIG_DIR` moves Claude Code's
folder, `CODEX_HOME` moves Codex's, `XDG_CONFIG_HOME` moves every
program that follows the XDG Base Directory Specification (the Linux
convention for where a program keeps its settings, data, state and cache).
The collectors look for the names of such variables in the files that set
a user's or the system's environment and warn when they find one. Read this
page when a collection log or the summary reports a relocation hint, or
when you expect an agent's files and the collection has none.

## What the warning means

A relocation hint is one line of the log, and one entry in
`collection.json`, for each place a startup file names a variable
that can move an agent's files:

```text
relocation hint: /home/alice/.bashrc:41 names CLAUDE_CONFIG_DIR (claude-code)
```

It gives the file, the line number, the variable and the agent. It says:

- The name appears on that line of that file, bounded by characters that
  cannot be part of a name, without regard to case.
- The agent's files may therefore be somewhere the catalog does not look,
  so the collection can be missing them silently.

It does not say:

- That the variable is set. The line may be a comment, an `unset`, a
  branch of a conditional that did not run, or a mention of the name
  inside a longer command.
- What the value is. The collectors never evaluate a startup file and
  never print the line they matched; the `value` field of each hint in
  `collection.json` is empty.
- That the agent's files moved. A variable set in a startup file may have
  been overridden by the process environment, a service definition or
  the registry, and an agent may ignore the variable on the platform you
  are looking at.
- That the variable is absent when there is no hint. The search reads files
  only. A value set in the Windows registry, in a service or container
  definition, by a launcher or by another process that started the agent
  is not found.

The file that holds each hint is collected, so you can read the line
later. It is attributed to the agent `shared` because it belongs to no
single agent, and it is flagged `secret: true` because startup files
often hold keys; `--no-secrets` leaves it out as `skipped_secret`, and
the hint is still recorded. A file the catalog already collects (a
`settings.json` that names a variable, for example) keeps the catalog's
row and gets no second one.

A hint for a `shared` variable (the `XDG_*` names and the variables
that move the startup files themselves, such as `ZDOTDIR`) is a weaker
signal than one naming an agent's own variable. Many programs honour the
`XDG_*` variables and a user may set them for reasons that have nothing to
do with any agent, so check what the line says before you act on it.

## Read the assignment in the collected file

Extract the file from the archive. Its path in the archive is
`fs/<original path>` ([Archive layout in output.md](output.md#archive-layout)),
and the log line names the original path and the line number. Look for
the form that matches the file's shell:

| Syntax | Used by |
| --- | --- |
| `export NAME=value`, or `NAME=value` then `export NAME` | sh, bash, zsh, ksh, dash; `/etc/environment` and `.pam_environment` take `NAME=value` alone |
| `set -gx NAME value`, or `SETUVAR --export NAME:value` in `fish_variables` | fish |
| `setenv NAME value` | csh and tcsh |
| `$env.NAME = "value"` | Nushell |
| `$NAME = "value"` | xonsh |
| `$env:NAME = "value"`, or `set E:NAME` in older scripts | PowerShell profiles |
| `<key>NAME</key>` with the value in the next `<string>` | launchd property lists |
| `[env]` table with `NAME = "value"` | `mise.toml` |

Then check three things before you believe the value:

- A conditional. An assignment inside an `if`, a `case` or a test on the
  host name, the terminal or the operating system applies only when the
  condition holds. Read the lines above it.
- A sourced script. A line such as `. ~/.config/agent-env.sh` or
  `source ~/work/env` sets variables from another file. If the hint is in
  a file that names such a script, collect the script too (it may be in a
  directory the catalog does not reach) and read it the same way.
- A later line. A second assignment or an `unset` further down wins over
  the first.

## Confirm the live value on a running host

When the host is still up, check what a new login of the user sees, through
the EDR remote shell, as the user who ran the agent (or as an
administrator acting for that user). These read only, but a login shell
runs the user's startup files, which are the files you want to be careful
with; prefer reading the collected file when it answers the question.

| Platform and shell | Command |
| --- | --- |
| Linux or macOS, bash or zsh | `sudo -u USER -i printenv NAME` |
| Linux or macOS, fish | `sudo -u USER fish -l -c 'printenv NAME'` |
| Linux, the user's systemd session | `sudo -u USER XDG_RUNTIME_DIR=/run/user/UID systemctl --user show-environment` |
| macOS, the user's login session | `sudo launchctl asuser UID launchctl getenv NAME` |
| Windows, PowerShell | `[Environment]::GetEnvironmentVariable('NAME', 'User')`, then `'Machine'` |
| Windows, another user's registry hive | `reg query "HKU\<SID>\Environment" /v NAME` (the hive must be loaded) |

Replace `USER`, `UID`, `SID` and `NAME`. An empty result means the variable
is not set in that place, not that the agent never saw it. The environment
of a process that is already running can differ from all of these; reading
it is a task for the EDR (its process view) and is out of scope for the
collectors.

## Collect the moved store

The catalog tries every entry against every home directory, so a store that
the variable moved to another place is not collected. The collectors have no
option that takes a store path yet; until
[issue 456](https://github.com/seanthegeek/doubleagent/issues/456) lands,
make a second collection by giving the collector a root in which the store
sits where the agent's default folder would be:

1. Take the agent's default folder from the `Replaces` column of the
   `Relocation variables` table in the agent's research document
   (linked from the table below), such as `.claude` for
   `CLAUDE_CONFIG_DIR`. `--list` prints it as the third field of the
   variable's line.
2. Make an empty scratch directory outside the evidence and under it the
   path `home/USER/<default folder>`, where `USER` is the user who set the
   variable. On Windows, use `Users\USER\<default folder>`.
3. Put the moved store there without changing it. On a mounted disk image,
   copy it from the mount (`cp -a`, or `robocopy /E /COPY:DAT /R:0 /XJ` on
   Windows). On a live Linux host, mount it read-only instead, which copies
   nothing: `mount --bind SRC DEST`, then `mount -o remount,bind,ro DEST`.
4. Run the collector with that scratch directory as the root, and a
   second output directory or the same one (every file name carries the
   host name and the time of the run, so the archives stay beside the first):

   ```sh
   collectors/collect-agent-artifacts.sh -r SCRATCH -u USER -o OUTPUT -q
   ```

   ```powershell
   .\Collect-AgentArtifacts.ps1 -Root SCRATCH -Users USER -OutputDir OUTPUT -Quiet
   ```

The second archive's manifest paths begin with the scratch root, and its
`hostname` is the machine you ran it on. Keep a note of which variable, which
host and which original path the scratch directory stands for, since the
archive does not record it.

A store that holds project files rather than an agent's home (a directory
that has the agent's project-level files inside it, such as a `.claude`
folder, `AGENTS.md` or `.mcp.json`) needs no scratch root: give it to the
collector as a project with `-p DIR` (`-Project DIR`), which collects the
project-level files of the catalog from it. A path inside the evidence
is given with its mount point, as [options.md](options.md) describes.

## Variables the warning covers

Each agent's research document lists its variables, what each moves, and the
default folder it replaces, in its `Relocation variables` subsection.

| `agent-zero` | `A0_COMPUTER_USE_HOST_ARTIFACT_ROOT` | [agent-zero](../../research/agent-zero.md#relocation-variables) |
| `aider` | `AIDER_CHAT_HISTORY_FILE`, `AIDER_INPUT_HISTORY_FILE`, `AIDER_LLM_HISTORY_FILE`, `AIDER_ANALYTICS_LOG` | [aider](../../research/aider.md#relocation-variables) |
| `amp` | `AMP_HOME` | [amp](../../research/amp.md#relocation-variables) |
| `augment` | `AUGMENT_VFS_DIR` | [augment](../../research/augment.md#relocation-variables) |
| `autogen-studio` | `AUTOGENSTUDIO_APPDIR`, `AUTOGENSTUDIO_DATABASE_URI` | [autogen-studio](../../research/autogen-studio.md#relocation-variables) |
| `camel-ai` | `CAMEL_WORKDIR`, `CAMEL_LOG_DIR` | [camel-ai](../../research/camel-ai.md#relocation-variables) |
| `claude-code` | `CLAUDE_CONFIG_DIR`, `CLAUDE_CODE_TMPDIR` | [claude-code](../../research/claude-code.md#relocation-variables) |
| `claude-desktop` | `CLAUDE_USER_DATA_DIR` | [claude-desktop](../../research/claude-desktop.md#relocation-variables) |
| `cline` | `CLINE_DIR`, `CLINE_DATA_DIR`, `CLINE_SESSION_DATA_DIR`, `CLINE_DB_DATA_DIR`, `CLINE_TEAM_DATA_DIR`, `CLINE_CONNECTOR_DATA_DIR`, `CLINE_CONNECTOR_SETTINGS_PATH`, `CLINE_CONNECTORS_DB_PATH`, `CLINE_CRON_DB_PATH`, `CLINE_TASKS_DB_PATH`, `CLINE_PROVIDER_SETTINGS_PATH`, `CLINE_GLOBAL_SETTINGS_PATH`, `CLINE_MCP_SETTINGS_PATH`, `CLINE_HOOKS_LOG_PATH`, `WORKSPACE_STORAGE_DIR` | [cline](../../research/cline.md#relocation-variables) |
| `codex-cli` | `CODEX_HOME`, `CODEX_SQLITE_HOME`, `CODEX_ROLLOUT_TRACE_ROOT`, `CODEX_TUI_SESSION_LOG_PATH` | [codex-cli](../../research/codex-cli.md#relocation-variables) |
| `continue` | `CONTINUE_GLOBAL_DIR` | [continue](../../research/continue.md#relocation-variables) |
| `copilot-cli` | `COPILOT_HOME`, `COPILOT_PROVIDERS_CONFIG`, `COPILOT_PKG_CACHE_HOME`, `COPILOT_CACHE_HOME` | [copilot-cli](../../research/copilot-cli.md#relocation-variables) |
| `copilot-desktop` | `COPILOT_HOME`, `COPILOT_HOME_OVERRIDE` | [copilot-desktop](../../research/copilot-desktop.md#relocation-variables) |
| `crewai` | `CREWAI_STORAGE_DIR` | [crewai](../../research/crewai.md#relocation-variables) |
| `crush` | `CRUSH_GLOBAL_CONFIG`, `CRUSH_GLOBAL_DATA`, `CRUSH_CACHE_DIR`, `CRUSH_SKILLS_DIR` | [crush](../../research/crush.md#relocation-variables) |
| `cursor` | `CURSOR_DATA_DIR`, `CURSOR_CONFIG_DIR`, `CURSOR_WORKTREES_ROOT`, `CURSOR_SANDBOX_POLICY_DIR` | [cursor](../../research/cursor.md#relocation-variables) |
| `dify` | `DIFY_CONFIG_DIR` | [dify](../../research/dify.md#relocation-variables) |
| `factory-droid` | `FACTORY_HOME_OVERRIDE`, `FACTORY_LOG_FILE` | [factory-droid](../../research/factory-droid.md#relocation-variables) |
| `flowise` | `DATABASE_PATH`, `SECRETKEY_PATH`, `BLOB_STORAGE_PATH`, `LOG_PATH` | [flowise](../../research/flowise.md#relocation-variables) |
| `gemini-cli` | `GEMINI_CLI_HOME`, `GEMINI_CLI_TRUSTED_FOLDERS_PATH`, `GEMINI_CLI_ACTIVITY_LOG_TARGET`, `GEMINI_DEBUG_LOG_FILE`, `GEMINI_CONTEXT_TRACE_DIR`, `GEMINI_WRITE_SYSTEM_MD` | [gemini-cli](../../research/gemini-cli.md#relocation-variables) |
| `goose` | `GOOSE_PATH_ROOT`, `GOOSE_RECIPE_PATH`, `GOOSE_ADDITIONAL_CONFIG_FILES` | [goose](../../research/goose.md#relocation-variables) |
| `hermes` | `HERMES_HOME`, `HERMES_DATA_DIR_SUFFIX`, `HERMES_DESKTOP_USER_DATA_DIR`, `TERMINAL_SANDBOX_DIR` | [hermes](../../research/hermes.md#relocation-variables) |
| `kilo-code` | `KILO_CONFIG_DIR`, `KILO_CONFIG`, `KILO_DB` | [kilo-code](../../research/kilo-code.md#relocation-variables) |
| `kiro` | `KIRO_HOME`, `KIRO_ACP_RECORD_PATH` | [kiro](../../research/kiro.md#relocation-variables) |
| `langflow` | `LANGFLOW_CONFIG_DIR`, `LANGFLOW_KNOWLEDGE_BASES_DIR`, `LANGFLOW_FS_TOOL_BASE_DIR`, `LANGFLOW_LOG_FILE` | [langflow](../../research/langflow.md#relocation-variables) |
| `letta` | `LETTA_HOME`, `LETTA_LOCAL_BACKEND_DIR`, `LETTA_TRANSCRIPT_ROOT`, `LETTA_ARTIFACTS_DIR`, `LETTA_MODS_DIR`, `LETTA_DEBUG_FILE`, `LETTA_DIR`, `MEMGPT_CONFIG_PATH` | [letta](../../research/letta.md#relocation-variables) |
| `little-coder` | `PI_CODING_AGENT_DIR`, `LITTLE_CODER_MODELS_FILE`, `LITTLE_CODER_EXTENSIONS_DIR` | [little-coder](../../research/little-coder.md#relocation-variables) |
| `local-deep-research` | `LDR_DATA_DIR` | [local-deep-research](../../research/local-deep-research.md#relocation-variables) |
| `metagpt` | `METAGPT_PROJECT_ROOT` | [metagpt](../../research/metagpt.md#relocation-variables) |
| `muse-code` | `MUSE_AUTH_PATH`, `MUSE_PROJECTS_HOME` | [muse-code](../../research/muse-code.md#relocation-variables) |
| `n8n` | `N8N_USER_FOLDER`, `N8N_STORAGE_PATH`, `N8N_BINARY_DATA_STORAGE_PATH`, `DB_SQLITE_DATABASE`, `N8N_EVENTBUS_LOGWRITER_LOGFULLPATH` | [n8n](../../research/n8n.md#relocation-variables) |
| `nanobot` | `NANOBOT_AGENTS__DEFAULTS__WORKSPACE`, `OAUTH_CLI_KIT_TOKEN_PATH` | [nanobot](../../research/nanobot.md#relocation-variables) |
| `ollama` | `OLLAMA_MODELS` | [ollama](../../research/ollama.md#relocation-variables) |
| `open-interpreter` | `INTERPRETER_HOME`, `CODEX_SQLITE_HOME`, `OPEN_INTERPRETER_HOME`, `CODEX_ROLLOUT_TRACE_ROOT` | [open-interpreter](../../research/open-interpreter.md#relocation-variables) |
| `openclaw` | `OPENCLAW_STATE_DIR`, `OPENCLAW_HOME`, `OPENCLAW_PROFILE`, `OPENCLAW_CONFIG_PATH`, `OPENCLAW_WORKSPACE_DIR`, `OPENCLAW_OAUTH_DIR` | [openclaw](../../research/openclaw.md#relocation-variables) |
| `opencode` | `OPENCODE_CONFIG_DIR`, `OPENCODE_DB` | [opencode](../../research/opencode.md#relocation-variables) |
| `openhands` | `OH_PERSISTENCE_DIR`, `OPENHANDS_PERSISTENCE_DIR`, `OPENHANDS_CONVERSATIONS_DIR`, `FILE_STORE_PATH`, `OH_CANVAS_SAFE_STATE_DIR`, `OH_CONVERSATIONS_PATH`, `OH_BASH_EVENTS_DIR`, `OH_CONVERSATION_WORKTREE_ROOT`, `AUTOMATION_WORKSPACE_BASE`, `VITE_WORKING_DIR`, `OH_SECRET_KEY_PATH`, `OH_SESSION_API_KEY_PATH`, `LOG_DIR` | [openhands](../../research/openhands.md#relocation-variables) |
| `pearai` | `CONTINUE_GLOBAL_DIR`, `VSCODE_APPDATA`, `VSCODE_PORTABLE`, `VSCODE_EXTENSIONS` | [pearai](../../research/pearai.md#relocation-variables) |
| `pi` | `PI_CODING_AGENT_DIR`, `PI_CODING_AGENT_SESSION_DIR` | [pi](../../research/pi.md#relocation-variables) |
| `qwen-code` | `QWEN_HOME`, `QWEN_RUNTIME_DIR`, `QWEN_CODE_TRUSTED_FOLDERS_PATH`, `QWEN_CODE_MCP_APPROVALS_PATH`, `WEIXIN_STATE_DIR` | [qwen-code](../../research/qwen-code.md#relocation-variables) |
| `qwen-live` | `QWEN_LIVE_DATA_DIR`, `QWEN_LIVE_DISCOVERY_DIR` | [qwen-live](../../research/qwen-live.md#relocation-variables) |
| `shellgpt` | `CHAT_CACHE_PATH`, `CACHE_PATH`, `ROLE_STORAGE_PATH`, `OPENAI_FUNCTIONS_PATH` | [shellgpt](../../research/shellgpt.md#relocation-variables) |
| `tabby` | `TABBY_ROOT`, `TABBY_MODEL_CACHE_ROOT` | [tabby](../../research/tabby.md#relocation-variables) |
| `twinny` | `TWINNY_NODE_DIR` | [twinny](../../research/twinny.md#relocation-variables) |
| `vscode` | `VSCODE_PORTABLE`, `VSCODE_APPDATA`, `VSCODE_EXTENSIONS`, `VSCODE_AGENT_FOLDER` | [vscode](../../research/vscode.md#relocation-variables) |

Every agent document that reads a configuration or cache folder through an
`XDG_*` variable also lists those variables there, as `shared`. The `shared`
variables are `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`,
`XDG_CACHE_HOME`, and the variables that move the startup files themselves
(`ZDOTDIR`, `ENV`, `BASH_ENV`, `ZSH_CUSTOM`, `BASH_IT_CUSTOM`, `OMF_CONFIG`),
listed in [Relocation variables in `research/shared.md`](../../research/shared.md#relocation-variables).
`--list` prints the whole table, with the files that are searched for the
names, after the Docker volume names.

## Search the whole home

The search reads only the files that set an environment: the shell
startup files, `/etc/environment`, launchd property lists, `.envrc` and
`.env` and `mise` configuration in each project, and the like. A variable
can also be set in a helper script, a tool-version file or a file the
catalog collects. `--env-scan-home` (PowerShell: `-EnvScanHome`) adds a sweep
of every regular file in each home and project that the catalog did not
already match or an exclusion did not prune, and
records a hint for each file that names a variable. Symlinks are never
followed.

The sweep reads many files, so it adds time on a large home. A file bigger
than `--env-scan-max-size MB` (PowerShell `-EnvScanMaxSizeMB`; whole MiB,
default `1`, `0` for no limit) is not read and is counted in
`files_skipped_size` of `collection.json`. Binary files are read like any
other. The sweep itself only reads; the files it finds hints in are
collected under the usual `--max-file-size` cap.

## Claude Code config homes

A second Claude Code config home named `~/.claude-<name>` (such as
`~/.claude-work`) is collected as `claude-code`, but only by the
Claude-specific names inside it. One outside the home directory, or not
named `.claude-<name>`, is not found; collect it by hand.

Claude Code reads its whole config home from `CLAUDE_CONFIG_DIR` when that
variable is set, and a second account in such a directory is common. The
names collected there:

| Name | Flagged `secret: true` |
| --- | --- |
| `projects/` | no |
| `file-history/` | no |
| `history.jsonl` | no |
| `.claude.json*` | `.claude.json` only |
| `.credentials.json` | yes |
| `hfi-auth.json` | yes |
| `.session_ingress_token` | yes |
| `remote/.oauth_token` | yes |
| `remote/.api_key` | yes |
| `sessions/*.*.key` (the `<pid>.<sha256>.key` session keys) | yes |
| `daemon/control.key`, `daemon/pipe.key`, `daemon/roster.json` | yes |
| `backups/.claude.json.*` | yes |
| `ide/*.lock` | yes |
| `bridge-spawn/*/mcp.json` | yes |

The rest of a relocated home (`settings.json`, `todos/`, `plans/`,
`shell-snapshots/` and the other subdirectories) is not collected, because
a bare `.claude-*` entry would also take other tools' directories such as
`~/.claude-code-router` and `~/.claude-mem`.

## What comes next

[Issue 455](https://github.com/seanthegeek/doubleagent/issues/455) will
resolve plain values (the registry, `/etc/environment`, launchd property
lists) so the hint gives the path, and
[issue 456](https://github.com/seanthegeek/doubleagent/issues/456) will
collect and parse the store at that path in the same run. When they land,
the second collection above and most of the live checks are replaced
by the collector's output, and this page shrinks with them.
