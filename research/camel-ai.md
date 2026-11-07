# camel-ai

CAMEL is a multi-agent Python framework (package `camel-ai`) with toolkits for the terminal,
browser, Gmail and files. It came into the catalog from the agent framework sweep for
[issue #64](https://github.com/seanthegeek/doubleagent/issues/64); see
[`research/frameworks.md`](frameworks.md) for the frameworks that got no catalog line. It is
collected only, with no parser: CAMEL keeps no transcripts under the home.

## Source and evidence

[camel-ai/camel](https://github.com/camel-ai/camel), Apache-2.0, open source, commit
[`24fde60ad739b4be8f8f533fea9ba54256fbc4a2`](https://github.com/camel-ai/camel/commit/24fde60ad739b4be8f8f533fea9ba54256fbc4a2)
(2026-10-05), package `camel-ai` 0.2.91a7. All claims are from source.

CAMEL runs shell commands on the host: the terminal toolkit's `shell_exec`
uses `subprocess.Popen`
([terminal_toolkit.py:757](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L757),
[838](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L838)).

### Observed

- No real install: nothing was installed or run.

### Source reviews

- 2026-10-19:
  [`797296a`](https://github.com/camel-ai/camel/commit/797296a718ce856d9c9476e4bb69fedb71ab6382)
  (package version unchanged, 0.2.91a7) against
  [`24fde60`](https://github.com/camel-ai/camel/commit/24fde60ad739b4be8f8f533fea9ba54256fbc4a2),
  from source only. The home paths were unchanged; it added the caller-chosen tool log directory,
  the terminal toolkit's approval arguments and the absence of any sync
  ([Per-user storage](#per-user-storage)), and confirmed that there is nothing to parse.

## Collection

### Per-user storage

The home paths are the same at
[`797296a`](https://github.com/camel-ai/camel/commit/797296a718ce856d9c9476e4bb69fedb71ab6382)
([gmail_toolkit.py:1316](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/gmail_toolkit.py#L1316),
[skill_toolkit.py:206-207](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/skill_toolkit.py#L206-L207),
[runtime_utils.py:57](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/terminal_toolkit/runtime_utils.py#L57)).

| Path (home-relative, all OSes) | Holds | Default or opt-in |
| --- | --- | --- |
| `.camel/gmail_token.json` | Google OAuth token and refresh token, mode 0600 ([gmail_toolkit.py:1316](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/gmail_toolkit.py#L1316), [1390-1402](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/gmail_toolkit.py#L1390-L1402)) | default when `GmailToolkit` is used |
| `.camel/skills`, `.config/camel/skills` | user-scope skills, read ([skill_toolkit.py:201-209](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/skill_toolkit.py#L201-L209)) | read by default by `SkillToolkit` |
| `.camel/runtimes/{go,java}` | downloaded Go and JDK toolchains ([runtime_utils.py:57](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/terminal_toolkit/runtime_utils.py#L57), [terminal_toolkit.py:457-466](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L457-L466)) | default when the terminal toolkit needs them; not collected |

Everything else is relative to the working directory: `./workspace` for
the terminal toolkit
([terminal_toolkit.py:188-195](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L188-L195)),
`./camel_working_dir` for the file toolkits
([file_toolkit.py:69-73](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/toolkits/file_toolkit.py#L69-L73)),
`./chat_history.json` for `JsonStorage`
([json.py:71-73](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/storages/key_value_storages/json.py#L71-L73)),
and model logs only with `CAMEL_MODEL_LOG_ENABLED`, in `CAMEL_LOG_DIR` (default `camel_logs`)
([base_model.py:287-295](https://github.com/camel-ai/camel/blob/24fde60ad739b4be8f8f533fea9ba54256fbc4a2/camel/models/base_model.py#L287-L295);
the same at
[`797296a`](https://github.com/camel-ai/camel/commit/797296a718ce856d9c9476e4bb69fedb71ab6382),
[base_model.py:287-295](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/models/base_model.py#L287-L295)).
The browser `user_data_dir` is supplied by the caller; `save_memory(path)` and a `JsonStorage`
given a path write where the caller says
([chat_agent.py:1780](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/agents/chat_agent.py#L1780),
[json.py:72](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/storages/key_value_storages/json.py#L72)); and `ChatAgent` takes a
`tool_log_dir`, new since
[`24fde60`](https://github.com/camel-ai/camel/commit/24fde60ad739b4be8f8f533fea9ba54256fbc4a2),
that saves full tool outputs to a caller-chosen directory with no default
([chat_agent.py:640-644](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/agents/chat_agent.py#L640-L644)).

Nothing else about a run is recorded. The model is chosen in code. The terminal toolkit's
`safe_mode`, `allowed_commands` and `require_approval` callback are constructor arguments of
the calling program
([terminal_toolkit.py:143-148](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L143-L148),
[200-206](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L200-L206)), and no approval
decision is written anywhere. Nothing syncs or relays a session (not found in `camel/` at
[`797296a`](https://github.com/camel-ai/camel/commit/797296a718ce856d9c9476e4bb69fedb71ab6382),
grepped for `share_url`, `upload_trajectory` and `sync_to`), and there is no session to
delete.

#### Scratch directories

The terminal toolkit's `./workspace` and the file toolkits' `./camel_working_dir` are the
agent's own scratch space, created in the working directory
([terminal_toolkit.py:185-195](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L185-L195),
[file_toolkit.py:73](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/file_toolkit.py#L73)), unrecorded and under no
CAMEL-specific name, so no catalog line can find them.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `CAMEL_WORKDIR` | the working directory of the terminal and file toolkits, in place of `./workspace` and `./camel_working_dir` ([terminal_toolkit.py:191](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/terminal_toolkit/terminal_toolkit.py#L191), [file_toolkit.py:69](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/toolkits/file_toolkit.py#L69)) | | `camel-ai` |
| `CAMEL_LOG_DIR` | the model log directory, read when `CAMEL_MODEL_LOG_ENABLED` is set, default `camel_logs` ([base_model.py:295](https://github.com/camel-ai/camel/blob/797296a718ce856d9c9476e4bb69fedb71ab6382/camel/models/base_model.py#L295)) | | `camel-ai` |

Not relocation variables: `CAMEL_MODEL_LOG_ENABLED` (behaviour only), `MINIMAX_MCP_BASE_PATH`
(output directory of a third-party MCP server the toolkit launches).

#### Name collision

Apache Camel JBang also uses `~/.camel`
(`CAMEL_DIR = ".camel"` resolved under the home,
[CommandLineHelper.java:44](https://github.com/apache/camel/blob/43fb0300bd10fa74ef37f3eca82d20678b6d3350/dsl/camel-jbang/camel-jbang-core/src/main/java/org/apache/camel/dsl/jbang/core/common/CommandLineHelper.java#L44),
[221-222](https://github.com/apache/camel/blob/43fb0300bd10fa74ef37f3eca82d20678b6d3350/dsl/camel-jbang/camel-jbang-core/src/main/java/org/apache/camel/dsl/jbang/core/common/CommandLineHelper.java#L221-L222)),
so a bare `.camel` entry would collect JBang's state as `camel-ai`. The
catalog names only `.camel/gmail_token.json` and `.camel/skills`, plus the
CAMEL-only `.config/camel`.

#### Docker

No compose file or named volume.

### Credentials

- `.camel/gmail_token.json`: flagged. It is the one catalog line of real value: it shows that a
  Google account was granted to a CAMEL agent, which can then read and send mail as a tool.

### Exclusions

`.camel/runtimes` would need one only if `.camel` were collected whole; it
is not, so none.

### Project-local files

The working-directory files in [Per-user storage](#per-user-storage) are created wherever the
application runs, under no CAMEL-specific name, so they are not project catalog entries.

### Where the project path is recorded

Nowhere under the home.

## Parsing

CAMEL is `collected` only: there is nothing to parse, because it keeps no transcripts under the
home. Its chat history, model logs, tool logs and saved memory are written in the working
directory or wherever the calling program says ([Per-user storage](#per-user-storage)), so no
catalog line reaches them and a parser would have no file to read.

## Confidence

- High for every path (source).
