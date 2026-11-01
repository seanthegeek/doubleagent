# shellgpt

ShellGPT (`sgpt`, catalog name `shellgpt`) is an open-source Python command-line client for
OpenAI-compatible chat APIs. It answers one-shot prompts, generates shell
commands that the user can execute from a prompt, keeps named chat and REPL
sessions, and can let the model run shell commands through a "function"
plug-in. Its configuration lives under the home, but its chat history lives in
the system temporary directory. It stores a named chat or REPL session as the raw OpenAI `messages`
array it sends to the API, one JSON file per session, with no timestamps, model or working
directory. Paths cited are relative to the clone. The agent is parsed: the `shellgpt` parser
reads the chat files, four keys of the settings file, the function store and the response cache
([Parser plan](#parser-plan)).

## Source and evidence

[TheR1D/shell_gpt](https://github.com/TheR1D/shell_gpt), MIT, open source, commit
[`a082bd5327ce0c4ef5a0284d9060e833be9444a6`](https://github.com/TheR1D/shell_gpt/commit/a082bd5327ce0c4ef5a0284d9060e833be9444a6)
(2026-07-02), version 1.5.1
([`sgpt/__version__.py:1`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/__version__.py#L1)).
All claims from source. The older temp-directory layout is cited from the history commits
[`1465c4a`](https://github.com/TheR1D/shell_gpt/commit/1465c4af12ad313bd4948cf6434de06c6ca4e087)
(first in tag 0.7.3) and
[`a054a90`](https://github.com/TheR1D/shell_gpt/commit/a054a900958954400004e54bc7739ba94a75de1f)
(first in tag 0.9.0).

### Observed

- No real install.

### Source reviews

- 2026-10-21: TheR1D/shell_gpt at
  [a082bd5](https://github.com/TheR1D/shell_gpt/commit/a082bd5327ce0c4ef5a0284d9060e833be9444a6)
  (1.5.1): the parser now reads the `.sgptrc` permission and model keys, the function store and
  the response cache, so the Parser plan states those rows instead of proposing them; it added
  that the answer after a function call is cached on its own
  ([Response cache](#response-cache)) and the source-host statement
  ([Source host rows](#source-host-rows)).
- 2026-10-19: the head of the default branch is still
  [`a082bd5`](https://github.com/TheR1D/shell_gpt/commit/a082bd5327ce0c4ef5a0284d9060e833be9444a6),
  the commit above, so every path here stands. It added the finding that the response cache also
  holds chat answers ([Response cache](#response-cache)) and the permission and model settings of
  [Settings file and function store](#settings-file-and-function-store).

## Collection

### Per-user storage

#### Scratch directories

None. ShellGPT runs commands in the caller's
working directory and creates no directory of its own to work in
(`--install-functions` writes only into `functions/`).

#### Configuration folder

The config folder is `os.path.expanduser("~/.config")/shell_gpt` on every OS, Windows included:
there is no platform branch and no `XDG_CONFIG_HOME` support
([`sgpt/config.py:9-13`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L9-L13)).
So Windows hosts have `C:\Users\<u>\.config\shell_gpt`, and macOS uses `~/.config`, not `Library`.

- `~/.config/shell_gpt/.sgptrc`: `KEY=value` lines, one per setting
  ([`config.py:75-87`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L75-L87)).
  On first run every default in `DEFAULT_CONFIG` is written out, and keys added in later versions
  are appended to an existing file
  ([`config.py:18-46`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L18-L46),
  [`53-69`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L53-L69)).
  An environment variable of the same name overrides the file
  ([`config.py:89-94`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L89-L94)).
- `~/.config/shell_gpt/roles/<name>.json`: system roles, JSON with `name` and `role`
  ([`role.py:142-153`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/role.py#L142-L153)).
  Four defaults are created (`ShellGPT`, `Shell Command Generator`, `Shell Command Descriptor`,
  `Code Generator`;
  [`role.py:63-73`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/role.py#L63-L73));
  others come from `--create-role`. Overridable with `ROLE_STORAGE_PATH`
  ([`config.py:28`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L28)).
- `~/.config/shell_gpt/functions/*.py`: the function store, overridable with `OPENAI_FUNCTIONS_PATH`
  ([`config.py:32`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L32)).
  Every `.py` file here is imported and executed on every `sgpt` start
  ([`function.py:31-36`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/function.py#L31-L36),
  [`54-56`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/function.py#L54-L56)).
  `--install-functions` copies in `execute_shell.py`, which runs any command the model asks for with
  `shell=True`
  ([`llm_functions/init_functions.py:13-35`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/llm_functions/init_functions.py#L13-L35);
  [`common/execute_shell.py:18-25`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/llm_functions/common/execute_shell.py#L18-L25)).
  A file planted here is a code-execution and persistence point.

#### Chat history and response cache

Chat history is **not under the home**. `CHAT_CACHE_PATH` defaults to
`tempfile.gettempdir()/chat_cache` and the response cache `CACHE_PATH` to `gettempdir()/cache`
([`config.py:14-15`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L14-L15),
[`20-21`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L20-L21)).
That resolves to `/tmp/chat_cache` on Linux, `$TMPDIR/chat_cache` (`/var/folders/.../T/`) on macOS
and `%TEMP%\chat_cache` (`AppData\Local\Temp\chat_cache`) on Windows. Before 0.9.0 the defaults were
`gettempdir()/shell_gpt/chat_cache` and `gettempdir()/shell_gpt/cache`
([`1465c4a config.py:10-11`](https://github.com/TheR1D/shell_gpt/blob/1465c4af12ad313bd4948cf6434de06c6ca4e087/sgpt/config.py#L10-L11));
0.9.0 dropped the `shell_gpt` level
([`a054a90 config.py:15-16`](https://github.com/TheR1D/shell_gpt/blob/a054a900958954400004e54bc7739ba94a75de1f/sgpt/config.py#L15-L16)).
Because `.sgptrc` freezes the value written at first run and only appends missing keys, a user who
installed before 0.9.0 still writes to `/tmp/shell_gpt/chat_cache`; the current README also shows
that form
([`README.md:368-373`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/README.md#L368-L373)).
The official Docker image mounts a volume at `/tmp/shell_gpt`
([`Dockerfile:12-14`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/Dockerfile#L12-L14)).
Read `CHAT_CACHE_PATH` and `CACHE_PATH` from `.sgptrc` to find the real location.

- `<chat cache>/<chat id>`: one JSON file per `--chat` or `--repl` session, named exactly as the
  user typed the id, no extension
  ([`chat_handler.py:65-78`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/chat_handler.py#L65-L78)).
  The id `temp` is deleted at the start of each use
  ([`chat_handler.py:106-108`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/chat_handler.py#L106-L108)).
- `<cache>/<md5>`: one file per cached completion, one-shot prompts and chat answers alike,
  holding only the response text ([Response cache](#response-cache)); the name is an MD5 of the
  request arguments, so the prompt is not recoverable from it. The oldest files beyond
  `CACHE_LENGTH` (100) are deleted
  ([`cache.py:30-61`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/cache.py#L30-L61)).
  Caching is on by default
  ([`app.py:88-91`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L88-L91)).
  The catalog collects the pre-0.9.0 Windows form under `AppData/Local/Temp/shell_gpt`; the
  current form, `AppData/Local/Temp/cache`, is not a catalog line, and `cache` is too common a
  directory name in the temp directory to add as one without collecting other programs' caches.
  Reading `CACHE_PATH` from `.sgptrc` is the manual step.

#### One-shot prompts and the shell integration

One-shot prompts without `--chat` are not stored anywhere except as that response cache
([`app.py:222-239`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L222-L239);
[`default_handler.py:17-22`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/default_handler.py#L17-L22)).
The interactive `[E]xecute` prompt uses an in-memory `PromptSession` with no history file
([`app.py:241`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L241)).
The command line itself lands in the shell history.

`--install-integration` appends a `Shell-GPT integration` block binding Ctrl+L to `sgpt --shell` to
`~/.zshrc` or `~/.bashrc`
([`utils.py:66-87`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/utils.py#L66-L87);
[`integration.py:1-27`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/integration.py#L1-L27)).

#### Environment variables

`OPENAI_API_KEY`, `API_BASE_URL`, `CHAT_CACHE_PATH`,
`CACHE_PATH`, `ROLE_STORAGE_PATH`, `OPENAI_FUNCTIONS_PATH`, `USE_LITELLM`,
`DEFAULT_MODEL` and the rest of `DEFAULT_CONFIG`.

### Credentials

- `~/.config/shell_gpt/.sgptrc`: `OPENAI_API_KEY=<key>` in plain text. On first run sgpt prompts for
  the key and writes it, unless `OPENAI_API_KEY` is already in the environment
  ([`config.py:63-69`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L63-L69);
  [`README.md:10`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/README.md#L10)).
  The key is passed to whichever backend `API_BASE_URL` names
  ([`handler.py:15-21`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L15-L21)),
  so `API_BASE_URL` identifies the provider or proxy. No keychain use.
- With `USE_LITELLM=true` other providers' keys come from their own
  environment variables; nothing extra is written by sgpt.

### Exclusions

Nothing large. The function store and roles are a few KB. The response cache
is capped at 100 files by default.

### Project-local files

None. ShellGPT reads nothing from the working directory and records no
project.

### Where the project path is recorded

Nowhere: chat files carry no working directory. For discovery of the
history location rather than projects, `.sgptrc` keys `CHAT_CACHE_PATH`,
`CACHE_PATH`, `ROLE_STORAGE_PATH` and `OPENAI_FUNCTIONS_PATH` hold absolute
paths that may point outside the defaults.

## Parsing

### Transcript stores

- `<CHAT_CACHE_PATH>/<chat id>`: one file per session started with `--chat <id>` or `--repl <id>`,
  named exactly as typed, no extension, in the chat cache directory of
  [Chat history and response cache](#chat-history-and-response-cache) (by default under the temp
  directory, `.sgptrc` keeping whichever value was written at first run). Home-relative globs the
  catalog can reach: `AppData/Local/Temp/chat_cache/*` and
  `AppData/Local/Temp/shell_gpt/chat_cache/*` (Windows). On Linux and macOS the files are under
  `/tmp` or `/var/folders/*/*/T` and arrive only as loose input.
- The id `temp` is invalidated at each start, so it holds only the most recent throwaway session.

#### What survives a delete

ShellGPT has no delete command. What removes chat history is the `temp` chat id, invalidated at each
start; the cut on every write that keeps the system message and the last 100 other messages
([Record schema](#record-schema)); and, the main loss, the operating system's cleanup of its
temporary directory, where the chat files live by default. A per-prompt history file is not
stated. The response cache ([Response cache](#response-cache)) keeps the answers to chat turns
that the cut dropped, and to one-shot prompts that never had a chat file, as text with no prompt,
time or session, the newest 100 files kept; its current Windows path is named by no catalog line
([Chat history and response cache](#chat-history-and-response-cache)). Deletion markers are not
stated. The analyzer reads the chat files and, where it was collected, the response cache, a
secondary source joined on text only (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

The file is a single JSON array written by `json.dump` with default settings (one line,
ASCII-escaped)
([`chat_handler.py:72-78`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/chat_handler.py#L72-L78)).
Each element is an OpenAI chat message:

- `{"role":"system","content":str}`: always element 0, written only when the session is new
  ([`chat_handler.py:165-170`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/chat_handler.py#L165-L170)).
  The content is `"You are <role name>\n<role text>"`
  ([`sgpt/role.py:44`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/role.py#L44),
  [`role.py:150-153`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/role.py#L150-L153));
  sgpt itself parses the role name back from the first line
  ([`role.py:106-112`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/role.py#L106-L112)).
- `{"role":"user","content":str}`: the prompt. In REPL mode the first prompt is prefixed with any
  stdin or command-line prompt separated by three newlines
  ([`repl_handler.py:53-55`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/repl_handler.py#L53-L55)).
- `{"role":"assistant","content":null,"tool_calls":[{"id","type":"function","function":{"name","arguments"}}]}`:
  a function call; `arguments` is a JSON string
  ([`sgpt/handlers/handler.py:72-84`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L72-L84)).
  Only one call per turn (`parallel_tool_calls` is false;
  [`handler.py:118-121`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L118-L121)).
- `{"role":"tool","content":str,"tool_call_id":str}`: the function's return value
  ([`handler.py:98-100`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L98-L100)).
  For the bundled `execute_shell_command` it is `"Exit code: <n>, Output:\n<stdout+stderr>"`
  ([`llm_functions/common/execute_shell.py:18-25`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/llm_functions/common/execute_shell.py#L18-L25)).
- `{"role":"assistant","content":str}`: the full streamed response, appended after the call returns
  ([`chat_handler.py:56-61`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/chat_handler.py#L56-L61)).
  When a function was called it also contains the marker line `` > @FunctionCall `name(k="v")` ``
  and, with `SHOW_FUNCTIONS_OUTPUT=true`, the output in a fenced block
  ([`handler.py:89-95`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L89-L95)).
  The marker is preceded by a `"\n"`, which is always emitted because the check runs right after
  the assistant message is appended
  ([`handler.py:86-87`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L86-L87)).

So a turn with a function call is stored as user, assistant(tool_calls),
tool, assistant(content), in that order, because the handler mutates the
same list the session later writes.

Truncation: on every write the system message is kept and only the last `CHAT_CACHE_LENGTH`
(default 100) other messages survive
([`chat_handler.py:75-77`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/chat_handler.py#L75-L77);
[`config.py:22`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L22)).
Older turns of a long session are gone, and a cut can separate a `tool` message from its call.

### Joins and timestamps

- Session to project: none recorded.
- Tool call to result: `tool_calls[0].id` equals the next `tool`
  message's `tool_call_id`.
- No subagents. Session id is the file name.

No timestamp, model, temperature or cwd is stored. The only time is the
file's mtime, which is the last write. The file is rewritten in full each
turn.

### Other stores

No SQLite or binary store.

#### Response cache

`<CACHE_PATH>/<md5>` files hold only a response string with no prompt
([`sgpt/cache.py:30-41`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/cache.py#L30-L41)).
Not a transcript; because it also holds chat answers it is a secondary source
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)). `get_completion` of
every handler, the chat handler included, is wrapped by the cache decorator, which writes the full
response text to `<CACHE_PATH>/<md5 of the request>` unless it contains a function call
([`handler.py:102-110`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L102-L110);
[`sgpt/cache.py:30-44`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/cache.py#L30-L44)).
So the answers to chat turns that the 100-message cut dropped, and to one-shot prompts that have no
chat file, survive there as text with no prompt, time or session; they join on text only. The newest
100 files are kept
([`cache.py:46-61`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/cache.py#L46-L61)).
When the model calls a function, the handler runs it and calls the cached `get_completion` again
with `caching=False`, which skips the cache read but not the write
([`handler.py:155-166`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L155-L166);
[`cache.py:33-41`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/cache.py#L33-L41)). So the answer after a function call is
cached on its own, while the chat file stores it behind the `> @FunctionCall` marker
([Record schema](#record-schema)); the outer call's text holds the marker and is not cached. An
empty response is written as an empty file.

#### Settings file and function store

`.sgptrc` keys `OPENAI_USE_FUNCTIONS` (default `true`: function calls allowed),
`DEFAULT_EXECUTE_SHELL_CMD` (default `false`; `true` makes Execute the default answer) and
`SHELL_INTERACTION` (default `true`; `false` skips the prompt)
([`sgpt/config.py:29`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L29),
[33](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L33),
[38](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L38);
read at
[`app.py:60-61`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L60-L61),
[79-80](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L79-L80),
[247](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L247)).
The environment override of `SHELL_INTERACTION` reads the variable `"SHELL_INTERACTION "`, with a
trailing space, so only the file value takes effect in practice. The functions the model may call
are the `*.py` files in `~/.config/shell_gpt/functions/`, which `--install-functions` fills with
`execute_shell.py`
([`sgpt/llm_functions/init_functions.py:13-22`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/llm_functions/init_functions.py#L13-L22)),
and on macOS also `apple_script.py`
([`init_functions.py:29-35`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/llm_functions/init_functions.py#L29-L35)).
Every `*.py` file in that directory is loaded at start and offered to the model
([`sgpt/function.py:54-56`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/function.py#L54-L56)).
ShellGPT reads the file line by line, skipping blank lines and lines that start with `#`, and
splits each other line on its first `=`; a line with no `=` stops it with an error
([`config.py:82-87`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L82-L87)).

`.sgptrc` `DEFAULT_MODEL` (default `gpt-5.4-mini`,
[`config.py:25`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L25))
is the model used when no `--model` was given; nothing records a per-call choice.

### Format versions

No version field. Function calls arrived in 1.1.0
([`20ff0f2`](https://github.com/TheR1D/shell_gpt/commit/20ff0f2eeb26ee816bfd31d2fd0d67da6f1e861f))
using the legacy `functions` API: the call was stored as
`{"role":"assistant","content":"","function_call":{"name","arguments"}}` and the result as
`{"role":"function","content":str,"name":str}`, with no call id
([`handler.py:65-71`](https://github.com/TheR1D/shell_gpt/blob/6bd0bdebe1f321e397099e430e8ecd93beb965b6/sgpt/handlers/handler.py#L65-L71),
[`83`](https://github.com/TheR1D/shell_gpt/blob/6bd0bdebe1f321e397099e430e8ecd93beb965b6/sgpt/handlers/handler.py#L83)
at [`6bd0bde`](https://github.com/TheR1D/shell_gpt/commit/6bd0bdebe1f321e397099e430e8ecd93beb965b6),
the parent of the change below). 1.5.0 switched to `tool_calls` and `tool` messages
([`4ea2f83`](https://github.com/TheR1D/shell_gpt/commit/4ea2f834cff5dc020dabb25bb02030c630cd2062),
"Migrate to OpenAI v2"). A parser must accept both: pair a `function` message with the immediately
preceding `function_call` by position. Before 1.1.0 files hold only system, user and assistant
messages.

### Secrets

The API key is never written into chat files; it is in `.sgptrc`
([`config.py:63-69`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L63-L69)), beside
the settings the parser reads ([Permission rows](#permission-rows)), so the parser reads only
those named keys and never puts another key, value or line of that file in a row.
Tool messages contain raw shell output and can contain anything the commands printed.

### Parser plan

Detection: any regular file directly under a `chat_cache` directory whose
content parses as a JSON array of objects with `role`.

| Column | Source |
| --- | --- |
| timestamp_utc | empty; the manifest `mtime` may be given to the last row only |
| session_id | file name |
| project_path | empty |
| git_branch | empty |
| turn_type | `user` → user; `assistant` with `content` → assistant; each `tool_calls[]` → tool_use; `tool` → tool_result; `system` → system |
| model | empty |
| tool_name | `tool_calls[].function.name` |
| tool_use_id | `tool_calls[].id` / `tool_call_id` |
| text | `content`; for tool_use, `shell_command` from the parsed `arguments` when present, else `arguments` |
| source_line | 1 for every row (single-line file); the array index can go in the text or a later column |

#### Permission rows

Per call: none recorded, and none asked for function calls. `handle_function_call` appends the call,
runs `get_function(name)(**dict_args)` and appends the result with no confirmation
([`sgpt/handlers/handler.py:64-100`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L64-L100)),
so every `tool_use` row in a chat file is a command that ran. The `--shell` prompt
`[E]xecute, [M]odify, [D]escribe, [A]bort` runs the generated command with `run_command` after the
answer is written and records the choice nowhere
([`sgpt/app.py:243-260`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L243-L260)),
nor does the REPL's `e` in the shell role
([`sgpt/handlers/repl_handler.py:56-59`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/repl_handler.py#L56-L59)).

Standing: the `.sgptrc` switches and the function store described in
[Settings file and function store](#settings-file-and-function-store). The parser yields one
`permission setting: <key>=<value> (from .config/shell_gpt/.sgptrc, file mtime)` row for each of
`OPENAI_USE_FUNCTIONS`, `DEFAULT_EXECUTE_SHELL_CMD` and `SHELL_INTERACTION` present in the file, in
file order, and one `permission setting: function <file name> (from .config/shell_gpt/functions,
file mtime)` row per file in the function store, without reading the code; each at its file's
modification time with an empty `session_id`. Source-derived; not observed on any real install.
`.sgptrc` is a secret glob, so a `--no-secrets` collection lacks it, and an environment variable
that overrode a key at run time leaves no trace in the file.

#### Model rows

ShellGPT records no model in the chat cache, so no chat row names one. The configured default is
`DEFAULT_MODEL` in `.sgptrc`
([Settings file and function store](#settings-file-and-function-store)). The parser yields one
`model: <DEFAULT_MODEL> (configured default, from .config/shell_gpt/.sgptrc, file mtime)` row at
the file's modification time, with the value in the `model` column; it is the default, not a
per-session model, since `--model` is recorded nowhere. Source-derived; not observed on any real
install.

#### Past-use rows and secondary sources

The chat files are the primary store. The response cache ([Response cache](#response-cache)) is
secondary: the parser reads every `shell_gpt/cache/<md5>` file the catalog collects (the pre-0.9.0
Windows `AppData/Local/Temp/shell_gpt/cache/`; the current `AppData/Local/Temp/cache` is no
catalog line) and yields each non-empty one as an `assistant` row with no timestamp, session or
project, so it matches a chat row on text alone and is kept as recovered when no chat row holds
it. The answer after a function call never matches, because the chat file stores it behind the
marker, and is shown twice. Source-derived; not observed on any real install.

#### Sync, relay and origin rows

The agent records none. ShellGPT records no sync state, because it has no feature that uploads a
session. Its only network client is the chat completion call: the OpenAI client pointed at
`API_BASE_URL`, or `litellm.completion` when `USE_LITELLM=true` (LiteLLM is a third-party library
that routes one call format to many model providers)
([`handler.py:15-33`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L15-L33)),
which sends the session's messages with each prompt and nothing else
([`handler.py:123-130`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/handler.py#L123-L130)).
The runtime dependencies are `openai`, `typer`, `click`, `rich`, `distro`, `pyreadline3` and
`prompt_toolkit`, with `litellm` optional
([`pyproject.toml:26-39`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/pyproject.toml#L26-L39));
no HTTP library, telemetry or crash reporter.

The command-line options for sessions are `--chat`, `--repl`, `--show-chat` and `--list-chats`, all
local
([`app.py:98-119`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/app.py#L98-L119)),
and the session store reads, writes, lists and deletes files in `CHAT_CACHE_PATH` only
([`chat_handler.py:65-95`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/handlers/chat_handler.py#L65-L95)).
No setting in `DEFAULT_CONFIG` names a sync, share or account
([`config.py:18-46`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/config.py#L18-L46)).
The terms `sync`, `share`, `upload`, `cloud`, `remote`, `publish`, `telemetry`, `analytics`,
`export`, `backup`, `team`, `account`, `login`, `requests`, `httpx`, `urllib`, `socket` and `sentry`
match nothing in `sgpt/`, and the only URLs in `sgpt/` and `pyproject.toml` are the project's own
GitHub links.

Two paths are outside this source: what LiteLLM itself sends when it is enabled (ShellGPT sets only
`suppress_debug_info` on it) and what a user-installed function under
`~/.config/shell_gpt/functions/` does, since each is arbitrary Python imported at start
([`function.py:31-36`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/sgpt/function.py#L31-L36)).

#### Scratch directory rows

None ([Scratch directories](#scratch-directories)).

#### Deletion rows

No delete command; see [Transcript stores](#transcript-stores) and
[What survives a delete](#what-survives-a-delete) for `temp`, and [Record schema](#record-schema)
for the truncation. The parser yields no deletion rows.

#### Bad-line rows

A truncated array yields the messages before the cut and one `system` row. A line of `.sgptrc`
with no `=` yields `parser: unparseable line <n> (no "=" separator; not shown, the file holds the
API key)`, the line itself left out because the file holds the key; the other lines are still
read. The response cache is one text per file with nothing to cut. A chat file, `.sgptrc` or
response cache file that cannot be read is one row `parser: <file> could not be read (<reason>)`,
`<file>` its name, the chat file's row carrying its session id.

#### Fixtures

Fixture (`chat_cache/deploy-check`, shown wrapped; the real file is one line):

```json
[{"role": "system", "content": "You are ShellGPT\nYou are programming and system administration assistant.\nYou are managing Linux/Ubuntu 24.04 LTS operating system with bash shell.\nAPPLY MARKDOWN formatting when possible."},
 {"role": "user", "content": "what is listening on port 8080"},
 {"role": "assistant", "content": null, "tool_calls": [{"id": "call_Q1w2e3r4", "type": "function", "function": {"name": "execute_shell_command", "arguments": "{\"shell_command\": \"ss -ltnp | grep 8080\"}"}}]},
 {"role": "tool", "content": "Exit code: 0, Output:\nLISTEN 0 4096 0.0.0.0:8080 0.0.0.0:* users:((\"python3\",pid=4242,fd=3))\n", "tool_call_id": "call_Q1w2e3r4"},
 {"role": "assistant", "content": "\n> @FunctionCall `execute_shell_command(shell_command=\"ss -ltnp | grep 8080\")` \n\nA python3 process (PID 4242) is listening on 8080."}]
```

Fixture (`.config/shell_gpt/.sgptrc`, a shortened file in the order of `DEFAULT_CONFIG`, with a
placeholder key):

```text
CHAT_CACHE_PATH=/tmp/chat_cache
CACHE_PATH=/tmp/cache
DEFAULT_MODEL=gpt-5.4-mini
DEFAULT_EXECUTE_SHELL_CMD=true
OPENAI_USE_FUNCTIONS=true
API_BASE_URL=default
SHELL_INTERACTION=false
OPENAI_API_KEY=sk-not-real
```

Fixture (`shell_gpt/cache/<md5>`, the whole file): `30G is free on /.`

#### Source host rows

The agent records no source-host integration. `github`, `gitlab`, `bitbucket`, `oauth`,
`installation`, `repositor`, `pull request` and `clone` match nothing in `sgpt/` at
[a082bd5](https://github.com/TheR1D/shell_gpt/commit/a082bd5327ce0c4ef5a0284d9060e833be9444a6),
and the only matches in `pyproject.toml` are the project's own homepage and repository links
([`pyproject.toml:58-60`](https://github.com/TheR1D/shell_gpt/blob/a082bd5327ce0c4ef5a0284d9060e833be9444a6/pyproject.toml#L58-L60)).

## Confidence

- High (source and history): the config folder, the `.sgptrc` format and key, roles, the function
  store, the chat and cache layout, the 0.9.0 change of temp path, the chat file location, array
  shape, message keys, ordering and truncation.
- Medium: that `/tmp` is the effective Linux location; `gettempdir` honours `TMPDIR`, `TEMP` and
  `TMP`, so a user with any of those set writes elsewhere.
- Medium: the pre-1.5.0 shape, read from the parent of
  [`4ea2f83`](https://github.com/TheR1D/shell_gpt/commit/4ea2f834cff5dc020dabb25bb02030c630cd2062) in
  history rather than a released package.
- Not determined: how long the chat files survive. `/tmp` is a tmpfs cleared at boot on many
  distributions and macOS purges old `/var/folders` content, but this depends on the host and was
  not checked.
