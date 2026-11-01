# open-interpreter

Open Interpreter (catalog agent `open-interpreter`) today is a terminal CLI that is a Rust fork
of OpenAI Codex CLI: it keeps Codex's protocol and file formats and moves the home to its own
directory, `~/.openinterpreter`. This document leans on [codex-cli.md](codex-cli.md) for
everything the fork shares and records only what was checked in the fork and what differs. The
Python tool of 2023-2025 that carried the name left its own paths, described under
[Legacy Python SDK and CLI (0.x)](#legacy-python-sdk-and-cli-0x). The agent is parsed:
transcripts are Codex rollouts, byte for byte, so the Codex parser
(`doubleagent/parsers/codex.py`) reads them through its shared `RolloutParser`; the legacy
tool's `conversations/*.json` files are collected but not parsed yet
([issue 70](https://github.com/seanthegeek/doubleagent/issues/70)).

## Source and evidence

[openinterpreter/openinterpreter](https://github.com/openinterpreter/openinterpreter) (the old
`open-interpreter` URL redirects there), Apache-2.0, open source, commit
[`2767e5f20d6927500b8f1938c773c61afb823245`](https://github.com/openinterpreter/openinterpreter/commit/2767e5f20d6927500b8f1938c773c61afb823245)
(2026-10-02). All claims are from source. Paths below are relative to `codex-rs/` unless they start
with another top-level directory.

The current Open Interpreter is a Rust fork of OpenAI Codex CLI, not the Python tool of 2023-2025:
"Open Interpreter is a fork of OpenAI's Codex"
([`README.md:46`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/README.md#L46)),
and it keeps Codex crate names, protocol names and file names on purpose
([`FORK_BRANDING.md:3-6`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/FORK_BRANDING.md#L3-L6),
[`67-78`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/FORK_BRANDING.md#L67-L78)).
The layout under the home directory is therefore the Codex layout described in
[codex-cli.md](codex-cli.md), moved to a different directory. The current source contains no
reference to the Python-era paths (`platformdirs` `open-interpreter` config, `profiles/*.yaml`,
`conversations/*.json`); a grep of the tree for them finds nothing. Those paths are still on disk
wherever the Python tool was used, so they were researched separately in the agent framework sweep
for [issue 64](https://github.com/seanthegeek/doubleagent/issues/64) (see
[frameworks.md](frameworks.md)) and are described under
[Legacy Python SDK and CLI (0.x)](#legacy-python-sdk-and-cli-0x).

### Observed

- No real install.

### Source reviews

- 2026-10-21: [openinterpreter/openinterpreter](https://github.com/openinterpreter/openinterpreter)
  at
  [`ac1fcc8`](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87),
  the commit of the 2026-10-19 review; source only. The shared Codex reader now reads the standing
  grants, the memory rollout summaries, the `thread_goal_updated` events and the `db-backups/`
  copies of `state_5.sqlite` for the fork too, project discovery skips the thread worktrees, and
  the [Source host rows](#source-host-rows) were added.
- 2026-10-19: [openinterpreter/openinterpreter](https://github.com/openinterpreter/openinterpreter)
  at
  [`ac1fcc8`](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87),
  the head of `main` on 2026-10-19 (workspace version 0.0.56,
  [`codex-rs/Cargo.toml:163`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/Cargo.toml#L163)),
  compared file by file with
  [`2767e5f`](https://github.com/openinterpreter/openinterpreter/commit/2767e5f20d6927500b8f1938c773c61afb823245)
  (0.0.55), the commit cited elsewhere in this document. Only four files differ: the Codex
  compatibility profile the fork announces to the model service moved from 0.156.1 to 0.159.0
  ([`codex-rs/product-info/src/lib.rs:12`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/product-info/src/lib.rs#L12)),
  with its tests and the version numbers, so every path holds at `ac1fcc8`. The fork has not taken
  Codex's changes after its snapshot, so the Codex findings of the same day
  ([Source reviews in `codex-cli.md`](codex-cli.md#source-reviews)) apply only where noted. It
  added the standing grants and scratch directories of [Per-user storage](#per-user-storage), the
  delete facts of [What survives a delete](#what-survives-a-delete), the fork's approval logging of
  [Other stores](#other-stores) and the `turn_context` difference of
  [Format versions](#format-versions).

## Collection

### Per-user storage

Home resolution
([`utils/home-dir/src/lib.rs:5-36`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/utils/home-dir/src/lib.rs#L5-L36),
[`67-90`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/utils/home-dir/src/lib.rs#L67-L90)):
when the running product is Open Interpreter, `INTERPRETER_HOME` if set (must exist), else
`dirs::home_dir()/.openinterpreter`. `CODEX_HOME` is deliberately ignored "so the two products stay
isolated". No OS branching: `~/.openinterpreter` on Linux and macOS,
`%USERPROFILE%\.openinterpreter` on Windows. The product is chosen by `codex-package.json`
`variant: "open-interpreter"` next to the binary, or an executable named `interpreter*` or `i`
([`product-info/src/lib.rs:180-218`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/product-info/src/lib.rs#L180-L218)).
The installers default to the same directory and, unlike the runtime, fall back to `CODEX_HOME`
([`scripts/install/install.sh:15-22`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/scripts/install/install.sh#L15-L22),
[`install.ps1:922-930`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/scripts/install/install.ps1#L922-L930)).

Inside `~/.openinterpreter`, the same names as Codex:

- `sessions/YYYY/MM/DD/rollout-<timestamp>-<thread uuid>.jsonl` and `archived_sessions/`
  ([`rollout/src/lib.rs:84-85`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/lib.rs#L84-L85),
  [`rollout/src/rollout_file_name.rs:67`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/rollout_file_name.rs#L67));
  rollouts may be zstd-compressed `.jsonl.zst`
  ([`rollout/src/compression.rs:25`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/compression.rs#L25),[`64`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/compression.rs#L64)).
- `session_index.jsonl` thread names
  ([`rollout/src/session_index.rs:21`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/session_index.rs#L21)).
- `history.jsonl` prompt history
  ([`message-history/src/lib.rs:52`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/message-history/src/lib.rs#L52),[`86`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/message-history/src/lib.rs#L86)).
- SQLite: `logs_2.sqlite`, `goals_1.sqlite`, `memories_1.sqlite`,
  `queue_1.sqlite`, `state_5.sqlite`, `thread_history_1.sqlite`
  ([`state/src/sqlite.rs:29-34`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/src/sqlite.rs#L29-L34)).
- `config.toml`, which also carries the fork's `harness` and `harness_guidance` keys
  ([`config/src/config_toml.rs:166-170`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/config/src/config_toml.rs#L166-L170));
  `log/`
  ([`core/src/config/mod.rs:4009`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/config/mod.rs#L4009))
  with `codex-tui.log`
  ([`tui/src/lib.rs:273`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/tui/src/lib.rs#L273));
  `shell_snapshots/`
  ([`core/src/shell_snapshot.rs:106`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/shell_snapshot.rs#L106));
  `memories/`, `memories_v2/`
  ([`protocol/src/memory_version.rs:17-22`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/memory_version.rs#L17-L22));
  `installation_id`
  ([`core/src/installation_id.rs:17`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/installation_id.rs#L17));
  `rules/`
  ([`core/src/exec_policy.rs:54`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/exec_policy.rs#L54));
  `version.json`
  ([`cli/src/doctor/updates.rs:36`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/cli/src/doctor/updates.rs#L36));
  `models-cache/<provider>/`
  ([`app-server/src/models.rs:36`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/app-server/src/models.rs#L36));
  `app-server-daemon/` and `packages/`
  ([`app-server-daemon/src/managed_install.rs:22-28`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/app-server-daemon/src/managed_install.rs#L22-L28),[`53`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/app-server-daemon/src/managed_install.rs#L53)).
- Fork additions:
  - `external_agent_session_imports.json`, the ledger of chats imported by `/import`
    ([`external-agent-sessions/src/ledger.rs:15`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-sessions/src/ledger.rs#L15),[`193`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-sessions/src/ledger.rs#L193))
    with `source_path`, `content_sha256`, `imported_thread_id`, `imported_at`
    ([`ledger.rs:24-30`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-sessions/src/ledger.rs#L24-L30)).
    Import reads `~/.claude` and `~/.cursor`
    ([`external-agent-migration/src/source/cla.rs:20`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-migration/src/source/cla.rs#L20),
    [`source/cur.rs:19`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-migration/src/source/cur.rs#L19))
    and writes the result as ordinary rollout items
    ([`external-agent-sessions/src/export.rs:58-73`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-sessions/src/export.rs#L58-L73)).
  - `.zcode/` (zcode harness emulation):
    `.zcode/cli/artifacts/sess_<id>/<call>-tool-result-<uuid>.json` for tool output too long to
    inline
    ([`core/src/tools/handlers/harness_aliases.rs:887-902`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/tools/handlers/harness_aliases.rs#L887-L902)),
    `.zcode/oi-current-file-hashes/` and `.zcode/oi-todos/<session>.json`
    ([`harness_aliases.rs:78-79`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/tools/handlers/harness_aliases.rs#L78-L79),[`2520-2523`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/tools/handlers/harness_aliases.rs#L2520-L2523)).
    The hash and todo caches go under `OPEN_INTERPRETER_HOME`, `INTERPRETER_HOME` or `CODEX_HOME`
    only when one is set
    ([`harness_aliases.rs:1460-1465`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/tools/handlers/harness_aliases.rs#L1460-L1465)),
    so on a default install only the artifacts directory is written.
  - Windows sandbox `.sandbox/`, `.sandbox-bin/`, `.sandbox-secrets/` as in Codex
    ([`windows-sandbox-rs/src/uninstall_windows.rs:106-111`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/windows-sandbox-rs/src/uninstall_windows.rs#L106-L111)).

Executables are installed to `~/.local/bin` and `%LOCALAPPDATA%\Programs\Open Interpreter\bin`
(installers, lines cited above and
[`install.ps1:941`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/scripts/install/install.ps1#L941));
the package itself lands in `~/.openinterpreter/packages/standalone`.

#### Legacy Python SDK and CLI (0.x)

Checked at tag `v0.4.2`, commit
[`13061d2ce0ac35344f04c5285e795415cccf259d`](https://github.com/openinterpreter/openinterpreter/commit/13061d2ce0ac35344f04c5285e795415cccf259d)
(2024-10-24), AGPL-3.0 at that tag. PyPI's last release, 0.4.3, has no git
tag; its sdist was downloaded and its path calls (`platformdirs.user_*`,
`expanduser("~")`, `get_storage_path(...)`) are identical to v0.4.2. The
base directory is `platformdirs.user_config_dir("open-interpreter")`
([oi_dir.py:3](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/utils/oi_dir.py#L3),
[local_storage_path.py:6-13](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/utils/local_storage_path.py#L6-L13);
pin `platformdirs = "^4.2.0"`,
[pyproject.toml:58](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/pyproject.toml#L58)),
which is `~/.config/open-interpreter` on Linux
([platformdirs unix.py:49-51](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/unix.py#L49-L51)),
`~/Library/Application Support/open-interpreter` on macOS
([macos.py:22-23](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/macos.py#L22-L23),
[40-42](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/macos.py#L40-L42)),
and `%LOCALAPPDATA%\open-interpreter\open-interpreter` on Windows (the
config directory is the local data directory, with the app name doubled as
author;
[windows.py:32-44](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/windows.py#L32-L44),
[60-62](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/windows.py#L60-L62)).
Under it:

- `conversations/<first words>__<Month_DD_YYYY_HH-MM-SS>.json`: the full
  message list, a JSON array, of every chat from the CLI and from the SDK's
  `interpreter.chat()`, by default (`conversation_history=True`)
  ([core.py:62-64](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/core/core.py#L62-L64),
  [261-290](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/core/core.py#L261-L290)).
  This is not a Codex rollout, and the analyzer's Open Interpreter parser
  does not read it.
- `profiles/default.yaml` and other `profiles/*.yaml`, `.py` or `.json`,
  created on first run; they may hold `llm.api_key`
  ([profiles.py:19-20](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/profiles/profiles.py#L19-L20),
  [613-626](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/profiles/profiles.py#L613-L626),
  [default.yaml:9](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/profiles/defaults/default.yaml#L9)).
- `skills/`: agent-saved Python skills in OS mode
  ([skills.py:26](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/core/computer/skills/skills.py#L26)).
- `models/`: downloaded `.llamafile` weights, gigabytes, only after the
  `--local` setup
  ([local_setup.py:393-398](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/local_setup.py#L393-L398),
  [51-63](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/local_setup.py#L51-L63)).
- `*.png` at the root: temporary HTML render images
  ([html_to_png_base64.py:21-29](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/core/computer/utils/html_to_png_base64.py#L21-L29)).

Outside the base directory, `~/.cache/open-interpreter/telemetry_user_id` and `contribute.json`
(`expanduser`, every OS) hold the telemetry UUID and the contribution opt-in
([telemetry.py:24-29](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/core/utils/telemetry.py#L24-L29),
[contributing_conversations.py:14-16](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/contributing_conversations.py#L14-L16)).
Older layouts stay in place, because the migration copies and never deletes:
`user_config_dir("Open Interpreter")` before 0.2.0 and
`user_config_dir("Open Interpreter Terminal")` in 0.2.0, each with `conversations/`, `profiles/` and
`config.yaml`
([profiles.py:682-686](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/profiles/profiles.py#L682-L686),
[709-739](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/profiles/profiles.py#L709-L739),
[756-765](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/terminal_interface/profiles/profiles.py#L756-L765)).
The catalog covers all of these since collector 1.10.0 (`.config/Open Interpreter*` and its macOS
and Windows equivalents); `open-interpreter` and `Open Interpreter` differ by hyphen and space, not
only by case, so [issue 29](https://github.com/seanthegeek/doubleagent/issues/29) does not apply.
The legacy tool runs shell commands through a subprocess, with `auto_run=False` by default
([shell.py:8-20](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/core/computer/terminal/languages/shell.py#L8-L20),
[core.py:46](https://github.com/openinterpreter/openinterpreter/blob/13061d2ce0ac35344f04c5285e795415cccf259d/interpreter/core/core.py#L46)).
The unreleased `development` branch (1.0.0, commit
[`fa06bfda18b70546baa85b9d314873e048f70c99`](https://github.com/openinterpreter/openinterpreter/commit/fa06bfda18b70546baa85b9d314873e048f70c99))
uses `~/.openinterpreter/default_profile.py`
([profiles.py:29-30](https://github.com/openinterpreter/openinterpreter/blob/fa06bfda18b70546baa85b9d314873e048f70c99/interpreter/profiles.py#L29-L30)),
inside the current catalog entry; its `api_key` field
([profiles.py:44](https://github.com/openinterpreter/openinterpreter/blob/fa06bfda18b70546baa85b9d314873e048f70c99/interpreter/profiles.py#L44))
is not flagged, since that branch was never released to PyPI.

#### Standing grants

At
[`ac1fcc8`](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87)
the fork keeps Codex's code for these and for the scratch directories, described in
[Standing grants and approval records](codex-cli.md#standing-grants-and-approval-records) and
[Scratch directories](codex-cli.md#scratch-directories) in `codex-cli.md`.

`rules/default.rules`, the exec policy file Codex appends "approve and remember" and network rules
to
([`codex-rs/core/src/exec_policy.rs:54-56`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/core/src/exec_policy.rs#L54-L56)),
and `config.toml` project `trust_level` and tool `approval_mode` entries
([`codex-rs/core/src/config/mod.rs:2373`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/core/src/config/mod.rs#L2373),
[`codex-rs/core/src/mcp_tool_call.rs:2291`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/core/src/mcp_tool_call.rs#L2291),
[`2334`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/core/src/mcp_tool_call.rs#L2334),
[`2369`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/core/src/mcp_tool_call.rs#L2369)).
Collected by the `open-interpreter|.openinterpreter` line.

#### Scratch directories

Thread worktrees in `.openinterpreter/worktrees/<4 hex>/<repository name>`, or
under `config.toml` `[desktop] git-worktree-root`
([`codex-rs/worktree/src/settings.rs:12`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/worktree/src/settings.rs#L12),
[`44`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/worktree/src/settings.rs#L44)).
The default location is excluded ([Exclusions](#exclusions)); project discovery reads
`.openinterpreter/sessions` and `archived_sessions` and, from collector 1.18.0, drops a `cwd` under
`<home>/.openinterpreter/worktrees`
([issue 222](https://github.com/seanthegeek/doubleagent/issues/222)), so only a worktree under a
configured `git-worktree-root` is discovered as a project. The `.zcode/cli/artifacts` spill
directory is the agent's own output, under the catalog line.

### Credentials

- `auth.json`: OpenAI API key and ChatGPT OAuth tokens
  ([`login/src/auth/storage.rs:155`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/login/src/auth/storage.rs#L155));
  store mode `file` (default), `keyring`, `auto` or `ephemeral`
  ([`config/src/types.rs:114-124`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/config/src/types.rs#L114-L124)).
- `credentials/kimi-code.json` (Kimi Code OAuth `access_token`,
  `refresh_token`) and `device_id`, fork-only, always files
  ([`login/src/kimi_code.rs:22-24`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/login/src/kimi_code.rs#L22-L24),[`39-40`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/login/src/kimi_code.rs#L39-L40),[`243-253`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/login/src/kimi_code.rs#L243-L253)).
- `.credentials.json`: MCP OAuth fallback when no keyring
  ([`rmcp-client/src/oauth.rs:815`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rmcp-client/src/oauth.rs#L815)).
- `secrets/` with age-encrypted `local.age`, `gateway_oauth.age` and `gateway_oauth.lock`, the same
  module also names `codex_auth.age` and `mcp_oauth.age`
  ([`secrets/src/local.rs:41-44`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/secrets/src/local.rs#L41-L44),[`154`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/secrets/src/local.rs#L154);
  [`login/src/gateway_auth_storage.rs:20-24`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/login/src/gateway_auth_storage.rs#L20-L24)).
- `.env` in the home is loaded into the process environment at start, minus `CODEX_*` names
  ([`arg0/src/lib.rs:300-312`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/arg0/src/lib.rs#L300-L312));
  it is the obvious place for provider API keys. The Codex catalog does not flag `.codex/.env`
  either.
- `config.toml` can embed `experimental_bearer_token` per provider
  ([`model-provider-info/src/lib.rs:171`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/model-provider-info/src/lib.rs#L171)).
- `.sandbox-secrets/` on Windows, as in Codex.
- Legacy Python tool: `profiles/*` in the base directory, and the older
  `Open Interpreter*/config.yaml`, which can hold `llm.api_key`.

### Exclusions

Same as Codex: `packages/` (managed binaries), `cache/`, `.tmp/`, `tmp/`, `plugins/cache`
([`core-plugins/src/store.rs:26`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core-plugins/src/store.rs#L26)),
`skills/.system`
([`skills/src/lib.rs:57`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/skills/src/lib.rs#L57)),
`worktrees/`
([`worktree/src/settings.rs:44`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/worktree/src/settings.rs#L44)),
`visualizations/`
([`tui/src/inline_visualization.rs:92`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/tui/src/inline_visualization.rs#L92)),
`.sandbox`, `.sandbox-bin`, plus the fork's `models-cache/`. `.zcode/cli/plugins/cache` is only
referenced as a display path for the emulated skill
([`harness_aliases.rs:2853-2858`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/tools/handlers/harness_aliases.rs#L2853-L2858));
nothing writes it. Keep `.zcode/cli/artifacts`: it holds full tool output. For the legacy Python
tool, `models/` in the base directory (llamafile weights).

### Project-local files

- `.openinterpreter/` per project replaces `.codex/` (config, hooks, rules)
  ([`config/src/loader/mod.rs:1121-1126`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/config/src/loader/mod.rs#L1121-L1126)).
  The `/import` repository scope still writes `<repo>/.codex/config.toml`, `.codex/agents`,
  `.codex/hooks.json`
  ([`external-agent-migration/src/service.rs:560`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-migration/src/service.rs#L560),[`651`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-migration/src/service.rs#L651),[`664`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-migration/src/service.rs#L664)).
- `AGENTS.md` and `AGENTS.override.md`
  ([`core/src/agents_md.rs:43-45`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/agents_md.rs#L43-L45)).
- Harness emulation reads other agents' project files: `.kimi/AGENTS.md`, `.kimi/skills`,
  `.claude/skills`, `.codex/skills`, `.agents/skills`
  ([`core/src/harness/kimi_cli.rs:956`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/harness/kimi_cli.rs#L956),[`1127-1133`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/harness/kimi_cli.rs#L1127-L1133)),
  and writes `.codewhale/instructions.md` in the cwd under the DeepSeek harness
  ([`core/src/harness/deepseek_tui.rs:559-568`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/harness/deepseek_tui.rs#L559-L568)).
- The Terminus-2 harness keeps tmux buffers in
  `${INTERPRETER_HOME:-${CODEX_HOME:-$PWD}}/.terminus-2`
  ([`core/src/harness/terminus_2.rs:1060-1063`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/harness/terminus_2.rs#L1060-L1063)),
  so in the project directory when neither variable reaches the tool shell.
- No per-project database.

### Where the project path is recorded

As in Codex: `session_meta` first line of every rollout, `payload.cwd`
([`protocol/src/protocol.rs:3117-3141`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/protocol.rs#L3117-L3141)),
and the threads table in `state_5.sqlite`. The existing Codex discovery (`grep -ho '"cwd":"[^"]*"'`
over `sessions/**/*.jsonl`) applies unchanged with `.openinterpreter` in place of `.codex`;
`.jsonl.zst` rollouts need zstd first.

A rollout `cwd` under `.openinterpreter/worktrees` (or a configured `git-worktree-root`) is a
scratch directory rather than a project ([Scratch directories](#scratch-directories)), and
discovery skips one under the default location;
`config.toml` `[projects."<path>"]` tables name the
projects the user trusted or distrusted.

## Parsing

### Transcript stores

- `.openinterpreter/sessions/YYYY/MM/DD/rollout-<YYYY-MM-DDThh-mm-ss>-<thread uuid>.jsonl`, one file
  per thread, and the same under `.openinterpreter/archived_sessions/`
  ([`rollout/src/lib.rs:84-85`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/lib.rs#L84-L85),
  [`rollout/src/rollout_file_name.rs:64-67`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/rollout_file_name.rs#L64-L67)).
  The directory date and the file name time are local time
  ([`rollout/src/recorder.rs:1700-1712`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/recorder.rs#L1700-L1712));
  record timestamps are UTC. A rollout may be stored as `.jsonl.zst`
  ([`rollout/src/compression.rs:25`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/compression.rs#L25),[`94`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/compression.rs#L94)).
- `.openinterpreter/history.jsonl`: one line per submitted prompt, all sessions
  ([`message-history/src/lib.rs:52`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/message-history/src/lib.rs#L52),[`86`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/message-history/src/lib.rs#L86)).
- `.openinterpreter/session_index.jsonl`: `{id, thread_name, updated_at}` per rename
  ([`rollout/src/session_index.rs:21-29`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/session_index.rs#L21-L29)).
- `.openinterpreter/external_agent_session_imports.json`: which rollouts
  were imported from Claude Code or Cursor ([Joins and timestamps](#joins-and-timestamps)).
- `.openinterpreter/.zcode/cli/artifacts/sess_<session id>/<call id>-tool-result-<uuid>.json`: full
  output of a tool call that was too long to inline under the zcode harness
  ([`core/src/tools/handlers/harness_aliases.rs:887-902`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/tools/handlers/harness_aliases.rs#L887-L902)).

#### What survives a delete

As in Codex
([What survives a delete in `codex-cli.md`](codex-cli.md#what-survives-a-delete)), `thread/delete`
removes the thread's rollouts, its index
lines and its rows in the SQLite stores, `logs_2.sqlite` included, and
with them the `session synced:` Sentry feedback marker
([`codex-rs/state/src/runtime/threads.rs:1153`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/state/src/runtime/threads.rs#L1153),
[`codex-rs/thread-store/src/local/delete_thread.rs:262`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/thread-store/src/local/delete_thread.rs#L262)).
It leaves `history.jsonl`, one line per submitted prompt with its session
id and time, and the import ledger
`external_agent_session_imports.json`, which names the rollouts imported
from Claude Code or Cursor. The memories files (off by default) and
`db-backups/` exist as in Codex
([Memories and database backups](#memories-and-database-backups)); what a delete does to them is
not stated in this document. No deletion marker is
written. The analyzer reads `history.jsonl`, `state_5.sqlite`,
`thread_history_1.sqlite` and the `db-backups/` copies of `state_5.sqlite` as secondary sources,
the memory rollout summaries as a source of their own and the import ledger to
mark imported rollouts
([Past use and secondary sources](#past-use-rows-and-secondary-sources); see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

Each line is `RolloutLine`: `timestamp`, optional `ordinal`, and the flattened item
([`history/src/lib.rs:277-284`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/history/src/lib.rs#L277-L284)),
serialised as `{"type": <snake_case variant>, "payload": {...}}` with an optional `metadata` on
`response_item`
([`history/src/rollout_payload.rs:22-62`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/history/src/rollout_payload.rs#L22-L62)).
`timestamp` is UTC, `YYYY-MM-DDThh:mm:ss.mmmZ`
([`rollout/src/recorder.rs:2054-2059`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/recorder.rs#L2054-L2059)).
Types: `session_meta`, `response_item`, `inter_agent_communication`,
`inter_agent_communication_metadata`, `compacted`, `turn_context`, `token_usage_record`,
`world_state`, `retained_context`, `security_risk_score`, `event_msg`, `realtime_item`.

- `session_meta` payload
  ([`protocol/src/protocol.rs:3117-3154`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/protocol.rs#L3117-L3154)):
  `session_id`, `id` (thread), `forked_from_id?`, `parent_thread_id?`, `timestamp`, `cwd`,
  `runtime_workspace_roots?`, `originator`, `cli_version`, `source`, `agent_nickname?`,
  `agent_role?`, `agent_path?`, `model_provider`, `base_instructions`; plus `git` with
  `commit_hash`, `branch`, `repository_url`
  ([`protocol.rs:3222-3228`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/protocol.rs#L3222-L3228),
  [`3423-3437`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/protocol.rs#L3423-L3437)).
  `originator` defaults to `codex_cli_rs` in the fork too
  ([`login/src/auth/default_client.rs:42`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/login/src/auth/default_client.rs#L42),[`64-68`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/login/src/auth/default_client.rs#L64-L68)),
  so only the file's location tells Open Interpreter from Codex.
- `turn_context`: `turn_id`, `cwd`, `model`, `approval_policy`, `sandbox_policy`,
  `permission_profile`, `network`, `timezone`, `effort` and more, the same field names as Codex's
  `turn_context`
  ([`protocol.rs:3287-3343`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/protocol.rs#L3287-L3343)).
  The emulated harness (`claude-code`, `kimi-code`, `zcode` and others) is a `config.toml` key
  ([`config/src/config_toml.rs:166-170`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/config/src/config_toml.rs#L166-L170))
  and is not recorded per turn.
- `response_item` payload, tagged by `type`
  ([`protocol/src/models.rs:1009-1011`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/models.rs#L1009-L1011)):
  `message` (`role`, `content` blocks, `phase?`;
  [`1020-1035`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/models.rs#L1020-L1035)),
  `reasoning` (`summary`, `content?`, `encrypted_content`;
  [`1047-1059`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/models.rs#L1047-L1059)),
  `local_shell_call` (`call_id`, `status`, `action`;
  [`1060-1072`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/models.rs#L1060-L1072)),
  `function_call` (`name`, `namespace?`, `arguments` JSON string, `call_id`;
  [`1073-1092`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/models.rs#L1073-L1092)),
  `function_call_output` (`call_id`, `name?`, `output`;
  [`1113-1132`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/models.rs#L1113-L1132)),
  `custom_tool_call`, `custom_tool_call_output`, `tool_search_call`, `tool_search_output`,
  `web_search_call`. The harness emulation renames tools (`Bash`, `Read`, `Grep` and so on in the
  claude-code harness,
  [`core/src/harness/claude_code.rs:1287`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/harness/claude_code.rs#L1287)),
  so `name` values are not the Codex set.
- `event_msg`: UI events (`task_started`, `task_complete`, `token_count`,
  ...), as in Codex.

`history.jsonl`: `{session_id, ts, text}`, `ts` epoch seconds
([`message-history/src/lib.rs:62-66`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/message-history/src/lib.rs#L62-L66),[`128-131`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/message-history/src/lib.rs#L128-L131)).

### Joins and timestamps

Session to project: `session_meta.payload.cwd`, updated per turn by `turn_context.payload.cwd`. Tool
call to result: `call_id`. Subagent to parent: `session_meta.payload.parent_thread_id` and
`forked_from_id`. Imported chats: `external_agent_session_imports.json` `records[]` with
`source_path` (the original `~/.claude/projects/...` or `~/.cursor/...` file), `content_sha256`,
`imported_thread_id` (the rollout's thread id) and `imported_at`, `source_modified_at` (epoch)
([`external-agent-sessions/src/ledger.rs:18-30`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-sessions/src/ledger.rs#L18-L30)).
A rollout whose thread id is in that ledger was written by `/import`
([`external-agent-sessions/src/export.rs:58-73`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/external-agent-sessions/src/export.rs#L58-L73)),
so its turns were authored in another agent and their record timestamps are import time.

### Other stores

Same SQLite set as Codex (`state_5.sqlite`, `logs_2.sqlite`, `thread_history_1.sqlite` and others;
[`state/src/sqlite.rs:29-34`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/src/sqlite.rs#L29-L34));
the rollouts are the primary record. zstd for `.jsonl.zst` rollouts.

`state_5.sqlite` and `thread_history_1.sqlite` are parsed as secondary
sources by the shared Codex reader
([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)); their record shapes, joins and
what they survive are in [Other stores in `codex-cli.md`](codex-cli.md#other-stores). The fork's
`threads` migrations that the parser reads match Codex's
([`state/migrations/0001_threads.sql:1-19`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/migrations/0001_threads.sql#L1-L19),
[`0007_threads_first_user_message.sql:1`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/migrations/0007_threads_first_user_message.sql#L1),
[`0025_thread_timestamps_millis.sql:1-2`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/migrations/0025_thread_timestamps_millis.sql#L1-L2)),
and the thread history migrations stop at 0006
([`state/thread_history_migrations/0006_thread_turn_ends.sql:1-3`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/thread_history_migrations/0006_thread_turn_ends.sql#L1-L3)):
there is no `started_at_ms` or `completed_at_ms` on `thread_items`, so
`created_at_ms`, the item's start or the rollout line's time
([`thread-store/src/local/thread_history.rs:431-438`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/thread-store/src/local/thread_history.rs#L431-L438)),
stamps every item row, and an `agentMessage` row precedes its rollout
twin by the time the message took to stream.

#### Approval decisions in the logs

At
[`ac1fcc8`](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87)
the fork differs from Codex: the core logs a submission with its kind only,
`operation=exec_approval`, never the decision
([`codex-rs/core/src/session/handlers.rs:421`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/core/src/session/handlers.rs#L421)),
so the `op=ExecApproval` rows the shared reader turns into `permission decision:` rows cannot
occur; the app server's `<- response:` line with the decision is still logged
([`codex-rs/app-server/src/outgoing_message.rs:481`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/app-server/src/outgoing_message.rs#L481)),
and the fork has no `guardian_review_feedback` table (its state migrations stop at
`0055_thread_attachments.sql`).

#### Memories and database backups

As in Codex, the memories files (`memories/`, off by default,
[`codex-rs/features/src/lib.rs:1148-1150`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/features/src/lib.rs#L1148-L1150))
and `db-backups/`
([`codex-rs/state/src/runtime/recovery.rs:12`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/state/src/runtime/recovery.rs#L12))
exist at
[`ac1fcc8`](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87);
both are collected.

### Format versions

Tracks upstream Codex; no fork-specific version marker in the records.
`cli_version` in `session_meta` is the fork's version. The record shapes are unchanged at
[`ac1fcc8`](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87);
`turn_context` still has `network` there
([`codex-rs/protocol/src/protocol.rs:3318`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/protocol/src/protocol.rs#L3318)),
unlike Codex at the commit of the same day's review.

### Secrets

Not in rollouts by design, but tool arguments and outputs can carry pasted
keys. `reasoning.encrypted_content` is an opaque provider blob. The
credential files (`auth.json`, `credentials/kimi-code.json`, `.env`) are
listed in [Credentials](#credentials).

### Parser plan

`OpenInterpreterParser` subclasses the shared `RolloutParser`, as `CodexParser` does, and reads
`.openinterpreter/sessions/` and `.openinterpreter/archived_sessions/` rollouts, plain or
`.jsonl.zst`, and `.openinterpreter/history.jsonl` with agent `open-interpreter`; the shared
`RolloutParser` reads `archived_sessions/` and `.jsonl.zst` for both agents.

| Column | Source |
| --- | --- |
| timestamp_utc | line `timestamp`; history `ts` (s) |
| session_id | `session_meta.payload.id` |
| project_path | `turn_context.payload.cwd`, else `session_meta.payload.cwd` |
| git_branch | `session_meta.payload.git.branch` |
| turn_type | `message` by role; `function_call`, `custom_tool_call`, `local_shell_call`, `web_search_call` → tool_use; `*_output` → tool_result; `reasoning` → thinking; `task_started`/`task_complete` → system |
| model | `turn_context.payload.model` |
| model row | `system` `model: <model>` at the first `turn_context` that names one, `model: <old> -> <new>` at each change (the shared Codex reader) |
| permission mode row | `system` `permission mode: approval=<approval_policy> sandbox=<sandbox_policy> profile=<permission_profile> network=<network>` at the first `turn_context` with any of them and `permission mode: <old> -> <new>` on every change, `-` for a missing field |
| tool_name, tool_use_id | `name`, `call_id` |
| text | message text, `arguments`/`input`/`action.command`, `output` |

#### Permission rows

The `permission mode` row of the table carries the per-turn policy. There is no per-call decision
row: the fork, like Codex, lists the approval request events among the transient events the rollout
never stores
([`rollout/src/policy.rs:140-204`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/rollout/src/policy.rs#L140-L204)).

The standing grants of `rules/*.rules` and `config.toml` ([Standing grants](#standing-grants))
give one `permission setting:` row per rule statement and per standing key, from the shared reader,
as in Codex ([Permission rows in `codex-cli.md`](codex-cli.md#permission-rows)); source-derived;
not observed on any real install. The per-call decisions the fork logs, and its lack of a
`guardian_review_feedback` table, are in
[Approval decisions in the logs](#approval-decisions-in-the-logs).

#### Model rows

The model row of the table, from the shared Codex reader.

#### Past-use rows and secondary sources

The rollouts are the primary store; `history.jsonl`, `state_5.sqlite` and
`thread_history_1.sqlite` are secondary sources, read as for Codex
([Past use and secondary sources in
`codex-cli.md`](codex-cli.md#past-use-rows-and-secondary-sources)).
Each record of the import ledger becomes a `system` row naming `source_path`. The memory
rollout summaries, the `db-backups/` copies of `state_5.sqlite`
([Memories and database backups](#memories-and-database-backups)) and the `thread_goal_updated`
events, which the fork persists in the rollout as Codex does
([`codex-rs/rollout/src/policy.rs:114`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/rollout/src/policy.rs#L114)), are read by the
shared reader with Codex's roles and rows
([Past-use rows in `codex-cli.md`](codex-cli.md#past-use-rows-and-secondary-sources));
source-derived; not observed on any real install.

#### Sync, relay and origin rows

As for Codex ([Sync, relay and origin in `codex-cli.md`](codex-cli.md#sync-relay-and-origin-rows)),
the fork records no session sync and has the same exits, unchanged: `/feedback` ("send logs to
maintainers";
[`tui/src/slash_command.rs:93`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/tui/src/slash_command.rs#L93))
still reports to OpenAI's Sentry project, not to an Open Interpreter one
([`feedback/src/lib.rs:56-57`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/feedback/src/lib.rs#L56-L57)),
attaches the thread's rollouts when the user agrees to upload logs
([`app-server/src/request_processors/feedback_processor.rs:200-215`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/app-server/src/request_processors/feedback_processor.rs#L200-L215)),
is on by default
([`core/src/config/mod.rs:4403-4407`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/config/mod.rs#L4403-L4407))
and logs `feedback event uploaded to Sentry` with the thread id
([`feedback/src/lib.rs:682-688`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/feedback/src/lib.rs#L682-L688))
into `.openinterpreter/logs_2.sqlite`
([`state/src/sqlite.rs:29`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/src/sqlite.rs#L29)),
the marker and the `session synced: feedback report uploaded to Sentry, ...` row Codex has, which
the shared reader yields for the fork too (fixture only), removed by the fork's thread delete as in
Codex
([`state/src/runtime/threads.rs:1153`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/src/runtime/threads.rs#L1153)).
`codex cloud` (here "Browse remote tasks";
[`cli/src/main.rs:239-241`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/cli/src/main.rs#L239-L241))
still targets `https://chatgpt.com/backend-api/` by default
([`core/src/config/mod.rs:4341-4343`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/config/mod.rs#L4341-L4343))
and writes the same `error.log` line in its working directory
([`cloud-tasks-client/src/http.rs:377`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/cloud-tasks-client/src/http.rs#L377),
[`916`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/cloud-tasks-client/src/http.rs#L916));
remote control keeps the `remote_control_enrollments` table
([`state/migrations/0024_remote_control_enrollments.sql:1-10`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/state/migrations/0024_remote_control_enrollments.sql#L1-L10));
the thread store is local only
([`core/src/config/mod.rs:597-603`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/core/src/config/mod.rs#L597-L603)).
The fork adds no share, sync or upload feature of its own (searched `sync`, `share`, `upload`,
`cloud`, `remote` in the crates absent upstream: `acp-server`, `chat-wire-compat`,
`external-agent-sessions`, `product-info`, `mcp-server`, `execpolicy-legacy`).

The fork records no relay or origin.

#### Scratch directory rows

Worktrees under `.openinterpreter/worktrees` ([Scratch directories](#scratch-directories)),
classed by the catalog rule; no row names them.

#### Deletion rows

No marker; as in Codex ([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

The shared reader's rows: a `thread_history_1.sqlite` row with a bad `item_json` gives one row per
record with the value in full
([Bad lines in `codex-cli.md`](codex-cli.md#bad-line-rows)).

The shared reader's `state_5.sqlite`, `thread_history_1.sqlite` and `logs_<N>.sqlite` rows for a
database that cannot be read or is not SQLite are those of
[Bad lines in `codex-cli.md`](codex-cli.md#bad-line-rows).

A rollout or `history.jsonl` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)`, as in
[Bad lines in `codex-cli.md`](codex-cli.md#bad-line-rows).

#### Fixtures

- `.openinterpreter/sessions/2026/10/01/rollout-2026-10-01T12-00-00-0199a1b2-0000-7000-8000-000000000001.jsonl`:

```json
{"timestamp":"2026-10-01T10:00:00.000Z","type":"session_meta","payload":{"session_id":"0199a1b2-0000-7000-8000-000000000001","id":"0199a1b2-0000-7000-8000-000000000001","timestamp":"2026-10-01T10:00:00.000Z","cwd":"/home/alice/proj","originator":"codex_cli_rs","cli_version":"0.9.0","source":"cli","model_provider":"kimi-for-coding","git":{"branch":"main"}}}
{"timestamp":"2026-10-01T10:00:01.000Z","type":"turn_context","payload":{"turn_id":"t1","cwd":"/home/alice/proj","approval_policy":"on-request","sandbox_policy":{"type":"workspace-write"},"model":"kimi-k3"}}
{"timestamp":"2026-10-01T10:00:01.100Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"list the files"}]}}
{"timestamp":"2026-10-01T10:00:03.000Z","type":"response_item","payload":{"type":"function_call","name":"Bash","arguments":"{\"command\":\"ls\"}","call_id":"call_1"}}
{"timestamp":"2026-10-01T10:00:04.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call_1","output":"README.md"}}
```

The import ledger `external_agent_session_imports.json`:

- `{"records":[{"source_path":"/home/alice/.claude/projects/-home-alice-proj/5b1c.jsonl","content_sha256":"ab12...","imported_thread_id":"0199a1b2-0000-7000-8000-000000000002","imported_at":1759312800}]}`

#### Source host rows

The agent records no source-host integration in a file the collector gathers (source at
[`ac1fcc8`](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87),
grepped for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor` and `clone`): as
in Codex ([Source host rows in `codex-cli.md`](codex-cli.md#source-host-rows)), `codex cloud` looks
up a server-side environment by the local repository's GitHub origin
([`codex-rs/cloud-tasks/src/env_detect.rs:50-63`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/cloud-tasks/src/env_detect.rs#L50-L63))
and the network proxy reads a GitHub token from the environment only
([`codex-rs/network-proxy/src/credential_broker/providers/github.rs:13-20`](https://github.com/openinterpreter/openinterpreter/blob/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87/codex-rs/network-proxy/src/credential_broker/providers/github.rs#L13-L20)).

## Confidence

- High: home resolution, `CODEX_HOME` being ignored, every file name in
  [Per-user storage](#per-user-storage) and [Credentials](#credentials) (constants in the cited files,
  identical to upstream Codex), the Kimi credential file, the import ledger and its `records` key
  (no serde rename on the struct); the rollout line envelope, timestamp format, payload field names
  (shared structs) and `input_text` content blocks
  ([`protocol/src/models.rs:876-880`](https://github.com/openinterpreter/openinterpreter/blob/2767e5f20d6927500b8f1938c773c61afb823245/codex-rs/protocol/src/models.rs#L876-L880)).
- High: the legacy Python tool's 0.4.x layout (source and the 0.4.3 sdist).
- Medium: `models-cache/` as an exclusion (written per provider by the app server, contents not
  inspected); `.terminus-2` landing in the project (depends on whether the env vars reach the tool
  shell, not traced); pre-0.2 installs of the legacy tool on macOS, because platformdirs before
  3.x put `user_config_dir` under `~/Library/Preferences` and that history was not read, so a
  `Library/Preferences/Open Interpreter*` location is unverified and has no catalog line.
- Not determined: whether `device_id` is sensitive (an identifier sent to Kimi, not a secret,
  flagged here for caution); whether the desktop or IDE surfaces of Open Interpreter exist and
  write elsewhere (none found in this repository); the exact tool names each emulated harness
  emits, which vary by harness module.
