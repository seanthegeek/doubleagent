# aider

Aider (catalog agent `aider`) is a terminal pair-programming CLI that works inside a git
repository (or the current directory) and keeps its chat and prompt history there as plain-text
files, with a few optional files under the home, so most of its evidence is project-level. It
shares no storage or format with another agent. Paths cited are relative to the clone. The agent
is parsed: the `aider` parser reads the chat history as the primary store and the input and LLM
histories as secondary sources ([Parser plan](#parser-plan)).

## Source and evidence

[Aider-AI/aider](https://github.com/Aider-AI/aider), open source, commit
[`5dc9490bb35f9729ef2c95d00a19ccd30c26339c`](https://github.com/Aider-AI/aider/commit/5dc9490bb35f9729ef2c95d00a19ccd30c26339c),
Apache-2.0
([`LICENSE.txt`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/LICENSE.txt)).
All claims from source; the config-key example is from the shipped docs in the same tree. The input
history format comes from the upstream prompt_toolkit `history.py`, which is not vendored.

### Observed

- No real install.

### Source reviews

- 2026-10-21: Aider-AI/aider at
  [5dc9490](https://github.com/Aider-AI/aider/commit/5dc9490bb35f9729ef2c95d00a19ccd30c26339c):
  the parser now yields `model:` rows from the announcement lines and `permission decision:` rows
  from the confirmation lines ([Model rows](#model-rows), [Permission rows](#permission-rows));
  added the exact form of a confirmation answer, the `/settings` output that is not an
  announcement, and the source-host statement ([Source host rows](#source-host-rows)).
- 2026-10-19: the head of [Aider-AI/aider](https://github.com/Aider-AI/aider) `main` is still
  [`5dc9490`](https://github.com/Aider-AI/aider/commit/5dc9490bb35f9729ef2c95d00a19ccd30c26339c),
  the commit cited throughout, so there is no drift. Added the confirmation lines and the startup
  model announcements of the chat history ([Record schema](#record-schema)), the `yes-always`
  standing setting, the `/paste` temporary directory and what `/clear` and `/reset` leave
  ([What survives a delete](#what-survives-a-delete)).

## Collection

### Per-user storage

Aider has no platform branching; everything is `Path.home()` (`%USERPROFILE%` on Windows), so the
layout is identical on Linux, macOS and Windows. No `XDG` use.

Home-level files, all optional:

- `~/.aider.conf.yml`: the config search is cwd, git root, home
  ([`aider/main.py:464-477`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L464-L477);
  [`aider/args.py:790-796`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L790-L796)),
  or `--config`. At any level it can set `yes-always: true`, a standing yes to every confirmation
  ([Confirmations](#aiderchathistorymd)).
- `~/.aider.model.settings.yml` and `~/.aider.model.metadata.json`: `generate_search_path_list`
  tries home, git root, then cwd or the `--model-settings-file`/`--model-metadata-file` value
  ([`main.py:305-310`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L305-L310),[`336-338`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L336-L338),[`397-399`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L397-L399);
  defaults
  [`args.py:121-131`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L121-L131)).
- `~/.env`: `load_dotenv_files` loads `~/.env`, `<git root>/.env`, then `.env` in cwd or
  `--env-file`
  ([`main.py:361-385`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L361-L385);
  [`args.py:31-32`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L31-L32),[`801-804`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L801-L804)).
  This is a shared dotfile, not aider-owned.
- `~/.aider/oauth-keys.env`: loaded first
  ([`main.py:370-374`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L370-L374)),
  written by the OpenRouter OAuth flow
  ([`aider/onboarding.py:363-367`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/onboarding.py#L363-L367)).
- `~/.aider/analytics.json`: anonymous `uuid` and disable flag
  ([`aider/analytics.py:139-152`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L139-L152)).
  `--analytics-log <file>` writes JSONL events wherever the user says
  ([`args.py:575-578`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L575-L578)).
- `~/.aider/caches/`: `model_prices_and_context_window.json`
  ([`aider/models.py:169-170`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/models.py#L169-L170)),
  `openrouter_models.json`
  ([`aider/openrouter.py:34-35`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/openrouter.py#L34-L35)),
  `versioncheck`
  ([`aider/versioncheck.py:12`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/versioncheck.py#L12)),
  `help.<version>/` llama-index embedding store
  ([`aider/help.py:93`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/help.py#L93)).
- History files default to the git root ([Project-local files](#project-local-files)); when aider runs
  outside a repository they land in the cwd
  ([`args.py:271-276`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L271-L276)),
  so `~/.aider.chat.history.md` and friends exist only for sessions started in the home directory.

No relocation environment variable; everything is `--option` or config-file driven. `pip`/`uv`
install directories are out of scope.

#### Scratch directories

None known: Aider works in the git root or the cwd and makes no working directory of its own; the
only directory it creates is a system temporary directory for an image pasted with `/paste`
([`aider/commands.py:1295-1298`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L1295-L1298)),
outside every catalog line. `/clear` and `/reset` delete no file; what remains is in
[What survives a delete](#what-survives-a-delete).

### Credentials

- `~/.aider/oauth-keys.env`: `OPENROUTER_API_KEY="..."` appended in plaintext
  ([`onboarding.py:365-367`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/onboarding.py#L365-L367)).
- `.aider.conf.yml` at any of the three levels accepts `openai-api-key`, `anthropic-api-key`,
  `api-key: provider=key` and `set-env`
  ([`args.py:69-110`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L69-L110);
  documented example
  [`aider/website/docs/config/aider_conf.md:81-84`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/website/docs/config/aider_conf.md#L81-L84),[`109`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/website/docs/config/aider_conf.md#L109)).
- `.env` files at home, git root and cwd carry `*_API_KEY` values by design.
- `.aider.model.settings.yml` has no key field; `extra_params` is passed to litellm and could carry
  one, so flagging it is harmless but not evidence-based.
- `.aider.llm.history` contains full request bodies; keys are not logged.
- No keychain use.

### Exclusions

`.aider/caches` (model price tables, embedding store, version stamp). In repositories:
`.aider.tags.cache.v<N>` tree-sitter tag cache, currently `v3`
([`aider/repomap.py:35`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/repomap.py#L35),[`43`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/repomap.py#L43),[`186`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/repomap.py#L186),[`218`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/repomap.py#L218)).

### Project-local files

Defaults placed in the git root, or cwd without git
([`args.py:271-276`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L271-L276),[`422-426`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L422-L426)):
`.aider.input.history` (prompt_toolkit `FileHistory`,
[`aider/io.py:356`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L356)),
`.aider.chat.history.md` (appended per session,
[`io.py:336`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L336)),
`.aider.llm.history` only when `--llm-history-file` is given
([`args.py:296-300`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L296-L300);
written at
[`io.py:755-764`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L755-L764)),
`.aiderignore`, `.aider.tags.cache.v3/`, and per-level `.aider.conf.yml`,
`.aider.model.settings.yml`, `.aider.model.metadata.json`, `.env`. `CONVENTIONS.md` is a documented
convention loaded with `--read`
([`aider/website/docs/usage/conventions.md`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/website/docs/usage/conventions.md)),
not a hard-coded name. Aider adds `.aider*` and `.env` to the repository `.gitignore` on first run
([`main.py:160-168`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L160-L168)),
so git-based enumeration misses all of this; the collector must walk the directory.

### Where the project path is recorded

Nowhere under home. Aider keeps no index of repositories; `analytics.json` holds only a UUID, and
the chat history lives inside each repository. Discovery sources for other agents (and shell
history) are the only way to find aider projects; the `# aider chat started at <timestamp>` headers
in `.aider.chat.history.md` date the sessions once found.

## Parsing

### Transcript stores

In the git root, else the cwd
([`aider/args.py:271-275`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L271-L275)):
`.aider.chat.history.md` (markdown), `.aider.input.history` (prompt_toolkit `FileHistory` text),
`.aider.llm.history` (text; default `None`, written only when `--llm-history-file` is set,
[`args.py:295-299`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L295-L299)).
One file holds every session in that repository, appended.

#### What survives a delete

Aider has no delete command: `/clear` and `/reset` empty the in-memory chat, leave every file and
write no marker
([`commands.py:439-443`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L439-L443)).
The three files are project-local, in the git root, so they go only with the files themselves or the
repository
([`args.py:270-275`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L270-L275)).
`.aider.input.history` is the per-prompt history file: one entry per prompt with its local time and
no session, which survives the chat history being deleted, truncated or redirected but not the
project ([Past use and secondary sources](#past-use-rows-and-secondary-sources)).
`.aider.llm.history`, written only when `--llm-history-file` is set, is a second copy of every
prompt sent to the model and every response, each with its own time. No other store of past use was
found. The analyzer reads `.aider.chat.history.md` as the primary store and the input and LLM
histories as secondary sources on every run (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

#### .aider.chat.history.md

`.aider.chat.history.md` (append-only,
[`aider/io.py:1117-1136`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L1117-L1136)):

- Session header: `\n# aider chat started at YYYY-MM-DD HH:MM:SS\n\n` (local time,
  [`:335-336`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L335-L336)).
  The only dated line in the file.
- User prompt: `\n#### <line1>  \n#### <line2>  \n`; empty input is `####` followed by a blank
  ([`:775-789`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L775-L789)).
  Slash commands appear the same way (`#### /add foo.py`).
- Tool and system output, including `Applied edit to <path>`, `Commit <hash> <msg>`, errors and
  confirmations: `> text  \n` blockquote lines
  ([`:966-973`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L966-L973),
  [`:995-999`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L995-L999),
  [`base_coder.py:2334`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L2334),
  [`repo.py:313`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/repo.py#L313)).
- Startup announcements: at the start of each run, and again after a
  `/model`, `/weak-model`, `/editor-model` or `/chat-mode` switch, Aider writes its announcement
  lines as blockquotes
  ([`base_coder.py:550-554`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L550-L554),
  [`main.py:1042`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L1042),
  [`1174-1180`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L1174-L1180)):
  `> Aider v<version>`, then `> Model: <name> with <format> edit format` (or `> Main model: ...`
  when a separate weak model is in use), with `, <n> think tokens`, `, reasoning <effort>`,
  `, prompt cache` or `, infinite output` appended when they apply,
  `> Editor model: <name> with <format> edit format` in architect mode, `> Weak model: <name>`, and
  `> Git repo: <dir> with <n> files` or `> Git repo: none`
  ([`base_coder.py:207-260`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L207-L260)).
  `/settings` prints the same announcement lines inside one multi-line message, and
  `append_chat_history` puts `> ` before the first line of a message only, so only its
  `> Aider v<version>` line is a blockquote and the model lines that follow it read as assistant
  text
  ([`commands.py:1432-1460`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L1432-L1460),
  [`io.py:1117-1121`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L1117-L1121)).
  `/model <name>` itself is a `#### /model <name>` user line
  ([`commands.py:87-112`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L87-L112)).
- Confirmations: `confirm_ask` writes one blockquote line `>
  <question> (Y)es/(N)o[/(A)ll][/(S)kip all][/(D)on't ask again] [<default>]: <answer>`, the answer
  as typed (`y`, `n`, `a`, `s`, `d`, or empty for the default), preceded by the subject (the
  command, path or URL) on blockquote lines of its own
  ([`io.py:807-858`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L807-L858),
  [`903-905`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L903-L905),
  [`922-923`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L922-L923)).
  The line is
  `f"{question.strip()} {res}"` with `res` reduced to the lower-case first letter of the answer,
  the default filled in for an empty one, so the bracketed default ends in `]:` and one of the five
  letters follows
  ([`io.py:900`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L900),
  [`904`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L904),
  [`922`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L922)).
  An answer taken from an earlier "All" or "Skip all" of the same group (several URLs or files
  asked about together) is also written as a `#### <question><answer>` user line before the
  blockquote
  ([`io.py:870-872`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L870-L872)).
  The questions include `Run shell command?`, `Create new file?`, `Add file to the chat?` and
  `Add URL to the chat?`
  ([`base_coder.py:977`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L977),
  [`1773`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L1773),
  [`2207`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L2207),
  [`2455`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L2455)).
  Under `--yes-always` (`yes-always: true` in a config file,
  [`args.py:760-764`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L760-L764))
  the answer `y` is written without asking, and `n` for a question that requires an explicit yes
  ([`io.py:866-867`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L866-L867)),
  so a line alone does not say whether a person answered. `Run shell command?` is such a
  question
  ([`base_coder.py:2456-2461`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L2456-L2461)),
  so under `--yes-always` a suggested shell command is declined with an `n` line. "Don't ask
  again" adds the question to an in-memory set for the run, and a later asking of the same
  question is declined without a line
  ([`io.py:269`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L269),
  [`821-824`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L821-L824),
  [`903`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L903)).
- Assistant: raw markdown with no prefix, `\n<content>\n\n`
  ([`:793-795`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L793-L795),
  called from
  [`base_coder.py:1829`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L1829)),
  so SEARCH/REPLACE edit blocks appear verbatim inside fenced code. The function-call fallback
  writes `json.dumps(args)`
  ([`:1834`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L1834)).
- Aider's own reader
  ([`utils.py:148-188`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/utils.py#L148-L188)):
  skip lines starting `# `; `> ` is tool output; `#### ` is user; anything else is assistant.

#### .aider.input.history

`.aider.input.history` (prompt_toolkit `FileHistory`,
[`io.py:19`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L19),[`356`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L356),[`740`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L740);
format `prompt_toolkit/history.py:298-307`): an entry is `\n# YYYY-MM-DD HH:MM:SS.ffffff\n` followed
by one `+<line>` per input line (local time).

#### .aider.llm.history

`.aider.llm.history`
([`io.py:754-764`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L754-L764)):
`TO LLM YYYY-MM-DDTHH:MM:SS\n` followed by `format_messages` output
([`utils.py:112-134`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/utils.py#L112-L134)):
a `-------` separator, then `ROLE <line>` per content line (`SYSTEM`, `USER`, `ASSISTANT`), and
`LLM RESPONSE <ts>\n` plus `ASSISTANT <line>` lines
([`base_coder.py:1793`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L1793),
[`:1822-1826`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L1822-L1826)).

### Joins and timestamps

The project is the directory containing the files (the git root). There is
no session id; synthesise one per `# aider chat started at` header. Input
history timestamps join to `#### ` prompts by order and text. No branch is
recorded; the model is named only in the startup announcement lines
([Record schema](#record-schema)), which carry no time of their own and
follow the session header.

### Other stores

None besides the input and LLM histories of [Transcript stores](#transcript-stores), and no SQLite
store. `.aider.tags.cache.v*` is a repository-map cache, not chat
([Exclusions](#exclusions)).

### Format versions

None; the format has not changed across sessions.

### Secrets

`.aider.llm.history` contains the complete system prompt with every added
file; `.aider.chat.history.md` contains any pasted secrets. Aider writes no
keys itself. The credential files are in [Credentials](#credentials).

### Parser plan

Globs: `**/.aider.chat.history.md`, `**/.aider.input.history`,
`**/.aider.llm.history` (project-level, so they arrive through the
collector's project catalog).

Rows: one per `#### ` block (user), per assistant run, per `> ` run
(tool_result); `# aider chat started` becomes `system`.

| Column | Source |
| --- | --- |
| timestamp_utc | last `# aider chat started at` header, or the matching `# ` input-history line, converted from local time |
| session_id | synthesised `<path>#<header timestamp>` |
| project_path | the directory holding the file |
| git_branch | empty |
| model | the new model on a `model:` row; empty on every other row |
| tool_name, tool_use_id | empty |
| text | prompt text, assistant text, blockquote text |

#### Permission rows

Each confirmation line ([Record schema](#record-schema)) is a `system` row after the
`tool_result` row of its blockquote run, at that run's time, with `source_line` the confirmation
line: `permission decision: <decision> (<question>) answer=<letter> default=<default>`, where
`<decision>` is `yes`, `no`, `all`, `skip all` or `no, don't ask again` for the letters `y`, `n`,
`a`, `s` and `d`. A `y` or `n` answer adds `; may be automatic (--yes-always)`, since
`--yes-always` writes the same line without asking. The subject stays in the `tool_result` row,
the blockquote text before the question. A question declined without a line after "Don't ask
again" yields nothing. Source-derived; not observed on any real install.

The `yes-always` key of a collected `.aider.conf.yml` is a standing setting, but the parser does
not read the file: it is YAML, which the analyzer has no reader for, so no `permission setting:`
row is written for it.

#### Model rows

A blockquote run that holds a `Model:` or `Main model:` announcement line
([Record schema](#record-schema)) is read for the model lines, so the `/settings` output and a
stray `Weak model:` line elsewhere are not. After the run's `tool_result` row the parser writes a
`system` row per change, with `source_line` the announcement line and the `model` column the new
model: `model: <name>` at a session's first announcement and `model: <old> -> <new>` on each later
one that names another main model; ` (editor model)` or ` (weak model)` follows the same text for
the `Editor model:` and `Weak model:` lines. A `Model:` line means the weak model is the main
model, so it sets the weak model without a row unless it replaces a separate one. The model state
starts afresh at each session header. The edit format and the options after it (think tokens,
reasoning effort, prompt cache) are not in the row. Source-derived; not observed on any real
install.

#### Past-use rows and secondary sources

Sources: `.aider.chat.history.md` is primary; `.aider.input.history` and
`.aider.llm.history` are secondary, parsed on every run, with each row's
own time and the default two-second window.

`.aider.chat.history.md` is the store of record: it is the only file with session headers and
assistant and tool output, and the one Aider itself reads back with `--restore-chat-history`
([`base_coder.py:519-520`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L519-L520)).
The input and LLM histories are secondary sources (files that repeat some of a primary store's
events, in the sense of
[doubleagent issue 122](https://github.com/seanthegeek/doubleagent/issues/122)):

- Every prompt typed at the terminal is written to both: prompt_toolkit's `FileHistory` on the
  prompt session appends it to the input history
  ([`io.py:355-356`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L355-L356)),
  and `get_input` then calls `user_input`, which writes the `#### ` lines
  ([`io.py:732-733`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L732-L733),
  [`:775-789`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L775-L789)).
  A `--message` prompt is added with `add_to_input_history`
  ([`main.py:1126-1127`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L1126-L1127),
  [`io.py:736-745`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L736-L745))
  and written to the chat history by the coder
  ([`base_coder.py:877-879`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L877-L879)).
  The parser stamps the chat row with the matching input-history entry's time, so the two rows carry
  the same text, synthesised session id, project path and timestamp, and pair exactly.
- Input-history entries with no chat twin: a `/run <command>` entry Aider adds when it runs a shell
  command it suggested, whose chat trace is a `> Running <command>` blockquote instead
  ([`base_coder.py:2471-2474`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L2471-L2474));
  every prompt after a chat-history write error, which sets `chat_history_file` to `None` for the
  rest of the run
  ([`io.py:1133-1136`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L1133-L1136));
  and every prompt of a run whose `--chat-history-file` pointed elsewhere, since the two paths are
  separate options
  ([`args.py:276-287`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L276-L287)).
  These rows are kept.
- What the input history survives is in [What survives a delete](#what-survives-a-delete).
- The LLM history's `TO LLM` blocks become `system` rows that no chat row repeats. Its
  `LLM RESPONSE` block is timestamped when the response arrived
  ([`io.py:754-764`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/io.py#L754-L764)),
  while the chat history's assistant row inherits the prompt's time and may be split by `> ` lines,
  so an `assistant` row pairs only for a reply of identical text that arrived within the two-second
  window. Expect the LLM history's rows to stay in the timeline.

#### Sync, relay and origin rows

Aider records no sync state, because it uploads no session: no command or setting sends a chat
history, input history, LLM history or conversation title anywhere beyond the prompt sent to the
model provider through litellm (the library Aider calls for every model API). The commands are all
local
([`commands.py`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L87-L1638));
`/save` writes `/add` and `/read-only` lines to a local file
([`commands.py:1497-1522`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L1497-L1522)),
`/copy` uses the clipboard, and `/clear` empties the in-memory message lists without touching the
history files
([`commands.py:435-437`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L435-L437)).
What does leave the host:

- Analytics, through PostHog (a hosted product-analytics service) at `https://us.i.posthog.com`; the
  client for Mixpanel, another such service, is commented out
  ([`analytics.py:55-57`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L55-L57),
  [`analytics.py:101-108`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L101-L108)).
  Opt-in: a random 10% of users are asked once, `--analytics` asks anyone, and analytics stay off
  unless the user said yes
  ([`analytics.py:119-135`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L119-L135),
  [`main.py:642-662`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/main.py#L642-L662)).
  Events carry event names, a model name (redacted to `<provider>/REDACTED` when it is not a known
  model), token counts, costs, edit format and system information
  ([`analytics.py:186-240`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L186-L240),
  [`base_coder.py:2113-2122`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L2113-L2122)),
  never prompt or reply text; the exceptions are the `message_send_exception` event's `str(err)`
  ([`base_coder.py:1511`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L1511))
  and PostHog's own exception autocapture
  ([`analytics.py:106`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L106)),
  which can carry whatever a provider error message quoted. The opt-in state is
  `~/.aider/analytics.json`, keys `uuid` (string), `permanently_disable` (boolean or `null`) and
  `asked_opt_in` (boolean)
  ([`analytics.py:137-184`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L137-L184)),
  collected by the `.aider` catalog line; it says analytics may have been on, not what any session
  sent. `--analytics-log <file>` appends each event as a JSON line with `event`, `properties`,
  `user_id` and `time` (epoch seconds) to a path the user chooses
  ([`args.py:574-578`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L574-L578),
  [`analytics.py:242-253`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/analytics.py#L242-L253));
  no catalog line can name that path, but an `analytics-log:` key in a collected `.aider.conf.yml`
  does. `--analytics-posthog-host` and `--analytics-posthog-project-api-key` redirect the events to
  another PostHog
  ([`args.py:585-594`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/args.py#L585-L594)).
- Crash and `/report` text, which goes nowhere by itself: Aider builds a prefilled GitHub new-issue
  URL (version, platform, the traceback with file paths reduced to base names, or the startup
  announcements) and opens it in the browser for the user to submit
  ([`report.py:37-90`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/report.py#L37-L90),
  [`report.py:94-151`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/report.py#L94-L151),
  [`commands.py:1555-1567`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/commands.py#L1555-L1567)).

The OpenRouter OAuth flow writes `~/.aider/oauth-keys.env` (see [Credentials](#credentials)) and
carries no session. Searched for `sync`, `share`, `upload`, `publish`, `cloud`, `remote`, `gist`,
`paste`, `export`, `backup`, `account`, `team`, `session_id`, `conversation_id`, `remote_id` and
outbound `requests`/`httpx` calls in `aider/` (the website tree aside); the only hosts contacted are
the model providers, PostHog, OpenRouter's auth and model-list endpoints, GitHub's Copilot token
endpoint when a `github_copilot/` model is used with `GITHUB_COPILOT_TOKEN`
([`models.py:950-983`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/models.py#L950-L983)),
litellm's price table and pages fetched by `/web`. No other agent's directory is read or written for
this. The parser yields no `session synced:` row.

No session is relayed or of remote origin.

#### Scratch directory rows

None ([Scratch directories](#scratch-directories)).

#### Deletion rows

No marker; the files outlive `/clear` and `/reset`
([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

A non-blank input-history or LLM-history line that fits neither format is its own `system` row
carrying the line in full (`error_rows`); the chat history's markdown has no line that fails to
parse, since every line is a header, a prompt, a blockquote or assistant text.

Each of the three files that cannot be read, or fails while being read, is one row
`parser: <file> could not be read (<reason>)`, `<file>` its name, after the rows of the lines read
before the failure. The chat history and input history read beside another file, for its session
ids and prompt times, give no row there; their own rows report them.

#### Fixtures

Fixture `.aider.chat.history.md`:

```markdown
# aider chat started at 2026-10-02 12:00:00

#### rename foo to bar  
Here is the change:

a.py
<<<<<<< SEARCH
foo
=======
bar
>>>>>>> REPLACE

> Applied edit to a.py  
> Commit abc1234 refactor: rename foo to bar  
```

Announcement and confirmation lines, after a session header and after a `/model` switch:

```markdown
> Aider v0.86.1  
> Main model: anthropic/claude-sonnet-4-5 with diff edit format, prompt cache  
> Weak model: anthropic/claude-haiku-4-5  
> Git repo: .git with 3 files  
> pytest -q  
> Run shell command? (Y)es/(N)o/(D)on't ask again [Yes]: n  
> Main model: claude-opus-4-5 with architect edit format  
> Editor model: claude-sonnet-4-5 with editor-diff edit format  
> Add URL to the chat? (Y)es/(N)o/(A)ll/(S)kip all/(D)on't ask again [Yes]: d  
```

The other two files:

- `.aider.input.history`: `\n# 2026-10-02 12:00:05.123456\n+rename foo to bar\n`
- `.aider.llm.history`: `TO LLM 2026-10-02T12:00:05\n-------\nSYSTEM Act as an expert\n-------\nUSER
  rename foo to bar\nLLM RESPONSE 2026-10-02T12:00:09\nASSISTANT Here is the change:\n`

#### Source host rows

The agent records no source-host integration. A grep of `aider/` (the website tree aside) at
[5dc9490](https://github.com/Aider-AI/aider/commit/5dc9490bb35f9729ef2c95d00a19ccd30c26339c) for
`github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and `clone`
found no account sign-in, repository list or remote conversation: the one OAuth flow is
OpenRouter's ([Credentials](#credentials)), the GitHub hit is the Copilot model provider, whose
`GITHUB_COPILOT_TOKEN` is read from the environment and exchanged in memory for an API key that is
never written
([`models.py:950-983`](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/models.py#L950-L983)),
and `git` use is the user's own repository.

## Confidence

- High: all file names and search orders (source); chat and LLM history formats (read from the
  writers).
- Medium-high: input history (upstream prompt_toolkit source, not pinned to Aider's dependency
  version). Local-time timestamps need the host time zone to convert exactly.
- Not determined: nothing material.
