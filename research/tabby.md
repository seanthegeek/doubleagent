# tabby

Tabby, from [TabbyML/tabby](https://github.com/TabbyML/tabby), is a self-hosted completion and
chat server with a `tabby-agent` language server and VS Code, IntelliJ, Vim and Eclipse
plugins. The editor plugins talk to the server and keep no transcripts: chat history is
server-side, in the server host's SQLite database and a daily event log. The `tabby` parser
reads that database, its pre-migration backups, the event log, the server's configured models and
the two editor plugins' inline-edit command histories.

## Source and evidence

[TabbyML/tabby](https://github.com/TabbyML/tabby) at
[`21b29048d7bcf6b94f9f482f2d0fd05efadfd19f`](https://github.com/TabbyML/tabby/commit/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f).
Source public: Apache-2.0 outside `ee/`, the Tabby Enterprise licence inside it
([LICENSE:3-7](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/LICENSE#L3-L7);
[ee/LICENSE:1-12](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/LICENSE#L1-L12)).
Rust server, TypeScript `tabby-agent` and VS Code client, Kotlin IntelliJ plugin. All claims from
source except the JetBrains settings directory, which is from JetBrains documentation.

### Observed

- No real install.

### Source reviews

- 2026-10-21: TabbyML/tabby at
  [21b2904](https://github.com/TabbyML/tabby/commit/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f):
  the parser now reads the pages, the source-host integrations, the configured models, both
  command histories and the Docker-volume layout, so the Parser plan says what it writes; added
  [Source-host integrations](#source-host-integrations), the page section attachment and the
  serialised form of the two command histories, and the
  [Source host rows](#source-host-rows) statement.
- 2026-10-19: the repository's HEAD is still
  [`21b2904`](https://github.com/TabbyML/tabby/commit/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f),
  the commit this document cites, so every citation holds. It added the AI-written pages and the
  two editor command histories of [Other stores](#other-stores), the delete of one question and
  answer pair ([What survives a delete](#what-survives-a-delete)) and the configured models of
  [Model rows](#model-rows).

## Collection

### Per-user storage

#### Scratch directories

The clones under `repositories/` ([Server root `~/.tabby`](#server-root-tabby)) are Tabby's own
working copies of the remote repositories it indexes, not the user's projects; the catalog excludes
them ([Exclusions](#exclusions)). No session records a local working directory.

#### Server root `~/.tabby`

On a developer workstation Tabby runs as that user (state under their home); in Docker the images
set `TABBY_ROOT=/data`
([docker/Dockerfile.cuda:88](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/docker/Dockerfile.cuda#L88)),
which is outside any home and not covered by a home catalog.

`TABBY_ROOT` if set, else `home::home_dir()/.tabby`
([crates/tabby-common/src/path.rs:5-14](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/path.rs#L5-L14)).
No platform branching: the same name under the profile on Windows. The root is created with mode
0700 on Unix
([crates/tabby/src/main.rs:61-68](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/main.rs#L61-L68)).
Contents
([path.rs:28-54](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/path.rs#L28-L54)):

- `config.toml`: server configuration, `[[repositories]]` and `[model.*.http]` blocks
  ([crates/tabby-common/src/config.rs:18-38](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L18-L38),
  [164-168](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L164-L168),
  [300-316](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L300-L316)).
- `usage_anonymous_id`: a UUID for telemetry
  ([crates/tabby-common/src/usage.rs:20-26](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/usage.rs#L20-L26)).
- `events/YYYY-MM-DD.json`: JSON Lines (despite the extension) of every completion, view, select,
  dismiss and chat event, written in all modes
  ([crates/tabby/src/services/event.rs:16-41](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/services/event.rs#L16-L41),
  [68-93](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/services/event.rs#L68-L93)).
  Completion events carry the full prompt, code segments and generated text
  ([crates/tabby/src/services/completion.rs:410-421](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/services/completion.rs#L410-L421)).
- `ee/db.sqlite` with `-wal`/`-shm` (release builds use the `prod` feature,
  [.github/workflows/release.yml:138](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/.github/workflows/release.yml#L138);
  source builds without it write `ee/dev-db.sqlite`)
  ([ee/tabby-webserver/src/path.rs:5-15](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/path.rs#L5-L15);
  [ee/tabby-db/src/lib.rs:165-176](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/lib.rs#L165-L176)).
  Users, chat threads, AI pages, completion statistics, integrations, OAuth and SMTP settings
  ([Transcript stores](#transcript-stores)).
- `ee/db.backup-YYYYMMDD.sqlite`: a full copy taken before each schema migration
  ([lib.rs:189-221](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/lib.rs#L189-L221)).
  Older chats can survive here ([Pre-migration backups](#pre-migration-backups)).
- `ee/jobs/<job id>/stdout.log` (`dev-jobs` in dev builds): background job output, repository
  indexing and sync
  ([path.rs:17-23](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/path.rs#L17-L23);
  [ee/tabby-webserver/src/service/background_job/helper/logger.rs:40-44](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/background_job/helper/logger.rs#L40-L44)).
- `models/<registry>/<model>/` (weights, `models.json`), relocatable with `TABBY_MODEL_CACHE_ROOT`
  ([path.rs:12-13](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/path.rs#L12-L13),
  [44-50](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/path.rs#L44-L50);
  [crates/tabby-common/src/registry.rs:39](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/registry.rs#L39),
  [113-116](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/registry.rs#L113-L116)).
- `index/`: the Tantivy code and document index
  ([path.rs:40-42](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/path.rs#L40-L42)).
- `repositories/<sanitised git url>/`: clones of indexed remote repositories; `file://` URLs are
  indexed in place
  ([config.rs:198-218](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L198-L218)).

The webserver (and so the database) runs unless the hidden `--no-webserver` flag is given
([crates/tabby/src/serve.rs:112-114](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/serve.rs#L112-L114),
[125-145](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/serve.rs#L125-L145)).

#### Client `~/.tabby-client/agent`

Used by `tabby-agent`, the language
server behind the VS Code, IntelliJ, Vim and Eclipse plugins, from
`os.homedir()` on every OS:

- `config.toml`: `[server] endpoint`, `token`, request headers, proxy
  ([clients/tabby-agent/src/config/configFile.ts:19-28](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/config/configFile.ts#L19-L28),
  [161-164](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/config/configFile.ts#L161-L164)).
- `data.json`: `anonymousId`, cached server-provided config per endpoint, ignored issues
  ([clients/tabby-agent/src/dataStore/dataFile.ts:47-50](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/dataStore/dataFile.ts#L47-L50);
  [dataStore/index.ts:17-21](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/dataStore/index.ts#L17-L21)).
- `logs/YYYYMMDD/HHMMSS-<pid>*.log` plus `logs/audit.json`, rotated at 10 MB, kept 30 days
  ([clients/tabby-agent/src/logger/fileLogger.ts:13-26](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/logger/fileLogger.ts#L13-L26)).

#### VS Code extension

`TabbyML.vscode-tabby`
([clients/vscode/package.json:2-3](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/package.json#L2-L3))
keeps its state in `globalState` (`server.serverRecords` with per-endpoint `token`,
`edit.recentlyCommand`,
[clients/vscode/src/Config.ts:41-43](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/Config.ts#L41-L43),
[55-66](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/Config.ts#L55-L66),
[139-145](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/Config.ts#L139-L145)),
that is in `state.vscdb` under the `vscode` entry; it writes no `globalStorage` files.
`edit.recentlyCommand` holds the recent inline-edit instructions
([Editor command histories](#editor-command-histories)).

#### IntelliJ plugin

The plugin persists `intellij-tabby.xml` and `intellij-tabby-command-history.xml`
([clients/intellij/.../settings/SettingsService.kt:11-13](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/intellij/src/main/kotlin/com/tabbyml/intellijtabby/settings/SettingsService.kt#L11-L13);
[.../inlineChat/CommandHistory.kt:9-12](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/intellij/src/main/kotlin/com/tabbyml/intellijtabby/inlineChat/CommandHistory.kt#L9-L12));
the settings hold `serverEndpoint` but no token
([.../settings/SettingsState.kt:12](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/intellij/src/main/kotlin/com/tabbyml/intellijtabby/settings/SettingsState.kt#L12)).
The IDE stores them in `options/` of its configuration directory: `~/.config/JetBrains/<product>`,
`~/Library/Application Support/JetBrains/<product>`, `%APPDATA%\JetBrains\<product>`
([JetBrains docs](https://www.jetbrains.com/help/idea/directories-used-by-the-ide-to-store-settings-caches-plugins-and-logs.html)).
The command history ([Editor command histories](#editor-command-histories)) is requested as a
project service
([.../inlineChat/InlineChatIntentionAction.kt:97](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/intellij/src/main/kotlin/com/tabbyml/intellijtabby/inlineChat/InlineChatIntentionAction.kt#L97))
under an `@Service` annotation that names no level; whether the IDE then stores
`intellij-tabby-command-history.xml` in `options/`, where the catalog line looks, or in the
project's `.idea/` is not determined from this source. The same
`options/` directory holds the settings of every other plugin, among them the Cody plugin's
`cody.xml` and its siblings ([JetBrains IDE settings in `cody.md`](cody.md#jetbrains-ide-settings)),
which is why the catalog lines name Tabby's `intellij-tabby*.xml` files rather than the directory.

### Credentials

- `~/.tabby/config.toml`: `api_key` for each HTTP model backend
  ([config.rs:300-316](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L300-L316)).
- `~/.tabby-client/agent/config.toml`: `server.token`, the user's Tabby API token
  ([configFile.ts:20-23](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/config/configFile.ts#L20-L23)).
- `~/.tabby/ee/db.sqlite` (and backups): credential columns in plain text, the password a hash,
  among them the GitHub or GitLab access token of each source-host integration
  ([Secrets](#secrets); [Source-host integrations](#source-host-integrations)). The same file holds
  the chats, so collect it unflagged and redact by column.
- VS Code: endpoint tokens in `globalState` (`state.vscdb`), see
  [VS Code extension](#vs-code-extension).
- Keychain: not used by any component.

### Exclusions

- `.tabby/models`: GGUF weights, gigabytes.
- `.tabby/index`: Tantivy index, rebuilt from sources.
- `.tabby/repositories`: git clones.

Keep `events/`, `ee/` (database, backups, job logs) and `config.toml`.
`.tabby-client/agent/logs` is bounded by rotation and records server
connection and completion request activity; keep it.

### Project-local files

None. Neither the server nor `tabby-agent` writes into repositories.
Repositories to index are named in `config.toml` or in the database.

### Where the project path is recorded

- `~/.tabby/config.toml`: `[[repositories]] git_url`; `file://` URLs are local project directories
  ([config.rs:164-168](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L164-L168),
  [216-218](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L216-L218)).
- `ee/db.sqlite`: `repositories.git_url`
  ([schema.sql:53-60](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L53-L60))
  and `provided_repositories.git_url`.
- `events/*.json`: `segments.git_url` and `segments.filepath` per completion
  ([crates/tabby-common/src/api/event.rs:81-98](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/api/event.rs#L81-L98));
  these are repository identities, not local roots.

## Parsing

### Transcript stores

- `.tabby/ee/db.sqlite` (`-wal`, `-shm`), and `.tabby/ee/db.backup-YYYYMMDD.sqlite` copies made
  before each migration
  ([ee/tabby-webserver/src/path.rs:9-15](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/path.rs#L9-L15);
  [ee/tabby-db/src/lib.rs:189-221](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/lib.rs#L189-L221)).
  Chat threads (`threads`, `thread_messages`), AI-written pages (`pages`, `page_sections`), and
  per-user completion events (`user_events`, `user_completions`). Source builds without the `prod`
  feature use `dev-db.sqlite`.
- `.tabby/events/YYYY-MM-DD.json`: JSON Lines, one `LogEntry` per line, a new file per UTC day
  ([crates/tabby/src/services/event.rs:68-93](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/services/event.rs#L68-L93)),
  no retention. Written whether or not the database is in use
  ([ee/tabby-webserver/src/webserver.rs:34-46](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/webserver.rs#L34-L46);
  [crates/tabby/src/serve.rs:139-152](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/serve.rs#L139-L152)).

#### What survives a delete

Deleting a thread (`deleteThread`,
[mod.rs:1529-1540](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/schema/mod.rs#L1529-L1540))
removes the row
([threads.rs:385-391](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L385-L391))
and its messages by cascade
([ee/tabby-db/schema/schema.sql:189](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L189));
a pre-migration backup taken before the delete keeps the row and its `is_ephemeral` flag
([Shared threads](#shared-threads)). Ephemeral threads untouched for 7 days are deleted by a
background job
([threads.rs:393-430](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L393-L430);
[background_job/db.rs:33-35](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/background_job/db.rs#L33-L35)),
so the pre-migration backups and the event log can hold chats and completions the live database no
longer has.

`deleteThreadMessagePair`
([ee/tabby-schema/src/schema/mod.rs:1508](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/schema/mod.rs#L1508))
removes one user message and the assistant answer after it
([ee/tabby-db/src/threads.rs:339-382](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L339-L382)),
leaving the thread; only a pre-migration backup keeps the pair. No writer deletes a user row
(`DELETE FROM users` and `delete_user` grepped in `ee/tabby-db/src`), although `threads.user_id`
cascades
([schema.sql:177](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L177)).
An AI-written page outlives the thread it came from ([Pages](#pages)). The pre-migration backups and
the event log repeat the live database's records and outlive its deletes
([Pre-migration backups](#pre-migration-backups);
[Event log against `user_events`](#event-log-against-user_events)).

The editor command histories ([Editor command histories](#editor-command-histories)) live on the
developer's machine, not the server, and survive every server-side delete; the user removes an
entry from the quick pick or IntelliJ's history list.

The analyzer reads the live database, its pages included, as primary and the backups, the event
log and the two command histories as secondary sources
([Past use and secondary sources](#past-use-rows-and-secondary-sources); see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

#### SQLite specifics

WAL journal
([lib.rs:165-176](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/lib.rs#L165-L176));
collect `-wal` or lose recent rows. `thread_messages.attachment` is TEXT JSON written with
`JSON_OBJECT`
([threads.rs:159-166](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L159-L166)).
Migrations are sqlx (`_sqlx_migrations`). The 7-day cleanup of ephemeral threads is above.

### Record schema

#### threads

Columns
([ee/tabby-db/schema/schema.sql:167-178](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L167-L178)):
`id` integer, `is_ephemeral` (sidebar chats start ephemeral,
[ee/tabby-webserver/src/service/thread.rs:123](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/thread.rs#L123)),
`user_id`, `created_at`, `updated_at`, `relevant_questions` (JSON array of strings).

#### thread_messages

Columns
([schema.sql:179-190](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L179-L190)):
`id`, `thread_id`, `role` (`user` or `assistant`,
[ee/tabby-schema/src/dao.rs:719-734](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/dao.rs#L719-L734)),
`content` (Markdown; assistant content is appended while streaming,
[ee/tabby-db/src/threads.rs:274](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L274)),
`code_source_id`, `attachment` JSON (`code[]`, `client_code[]`, `doc[]`, `code_file_list`,
[ee/tabby-db/src/attachment.rs:5-10](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/attachment.rs#L5-L10)),
`created_at`, `updated_at`. `code[]` items: `git_url`, `commit`, `language`, `filepath`, `content`,
`start_line`; `client_code[]`: `filepath`, `start_line`, `content`
([attachment.rs:84-100](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/attachment.rs#L84-L100)).
Messages alternate roles; two in a row with the same role are rejected
([threads.rs:150-153](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L150-L153)).
No model, tool calls or thinking are stored.

#### user_events

Columns
([schema.sql:116-123](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L116-L123)):
`user_id`, `kind` in `completion|chat_completion|select|view|dismiss`
([dao.rs:597-606](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/dao.rs#L597-L606)),
`created_at`, `payload` (pretty JSON of the `Event`,
[ee/tabby-webserver/src/service/event_logger.rs:21-25](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/event_logger.rs#L21-L25)).

#### Event log line

`LogEntry`
([crates/tabby-common/src/api/event.rs:106-111](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/api/event.rs#L106-L111)):
`user` (server user id or null), `ts` (Unix ms,
[:113-120](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/api/event.rs#L113-L120)),
`event`, an externally tagged snake_case enum
([:32-73](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/api/event.rs#L32-L73)):
`{"completion":{...}}` with `completion_id`, `language`, `prompt` (the full prompt sent to the
model), `segments` (`prefix`, `suffix`, `clipboard`, `git_url`, `declarations[]`, `filepath`,
[:81-98](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/api/event.rs#L81-L98)),
`choices[]` (`index`, `text`), `user_agent`; `view`, `select`, `dismiss` with `completion_id`,
`choice_index`, `view_id`, `elapsed`; `chat_completion` is empty
([crates/tabby/src/routes/chat.rs:83](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/routes/chat.rs#L83)).

#### Shared threads

Sharing a thread does not upload it anywhere: the thread stays in this server's database and the
share turns it from ephemeral into persisted. Every thread is created with `is_ephemeral` true
([ee/tabby-webserver/src/service/thread.rs:120-127](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/thread.rs#L120-L127)),
and the only writer that clears it is the `setThreadPersisted` GraphQL mutation
([ee/tabby-schema/src/schema/mod.rs:1542-1555](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/schema/mod.rs#L1542-L1555);
[thread.rs:138-143](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/thread.rs#L138-L143)),
which runs `UPDATE threads SET is_ephemeral = ?, updated_at = DATETIME('now')`
([ee/tabby-db/src/threads.rs:125-131](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L125-L131)).
The web UI calls it from the Share button of the answer-engine page and of the chat panel (the page
the IDE extensions embed), then copies a link of the form `<server>/search/<title slug>-<thread id>`
to the clipboard
([ee/tabby-ui/app/search/components/search.tsx:1098-1137](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-ui/app/search/components/search.tsx#L1098-L1137);
[:266-275](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-ui/app/search/components/search.tsx#L266-L275);
[ee/tabby-ui/components/chat/chat-panel.tsx:115-134](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-ui/components/chat/chat-panel.tsx#L115-L134)).
So `is_ephemeral = 0` means the owner shared the thread. A persisted thread is readable by every
user of the server, not only its owner
([ee/tabby-schema/src/policy.rs:31-45](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/policy.rs#L31-L45)),
is listed in the home page's all-threads feed
([ee/tabby-ui/app/(home)/components/thread-feeds/all-threads.tsx:35-42](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-ui/app/%28home%29/components/thread-feeds/all-threads.tsx#L35-L42)),
and is exempt from the 7-day cleanup ([What survives a delete](#what-survives-a-delete)). No column
records the share time or who opened the link: `updated_at` is set at the share and again whenever
`relevant_questions` is rewritten
([threads.rs:108-120](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L108-L120)),
so the share happened at or before it. Nothing unshares a thread: no caller sets `is_ephemeral` back
to true
([thread.rs:140](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/thread.rs#L140)
is the only call of `update_thread_ephemeral`). Deleting a thread is in
[What survives a delete](#what-survives-a-delete).

### Joins and timestamps

`thread_messages.thread_id → threads.id`; `threads.user_id → users.id` (`users.email`, `users.name`
attribute the chat,
[schema.sql:17-30](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L17-L30)).
`user_events.payload.completion.completion_id` joins `user_completions.completion_id` and the
`view`/`select`/`dismiss` events of the same completion. Project identity is the `git_url` and
`filepath` in attachments or `segments`; no local cwd is recorded. `page_sections.page_id →
pages.id` for pages.

Timestamps: SQLite columns are `DATETIME('now')` defaults or `%F %X` strings, both UTC without zone,
`YYYY-MM-DD HH:MM:SS`
([ee/tabby-db/src/lib.rs:307-310](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/lib.rs#L307-L310);
[user_events.rs:27-31](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/user_events.rs#L27-L31)).

### Other stores

#### Pages

`pages` and `page_sections`
([schema.sql:267-288](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L267-L288))
are AI-written documents: `pages(author_id, title, content, created_at, updated_at, code_source_id)`
and `page_sections(page_id, title, content, position, created_at, updated_at, attachment)`. A
section's `attachment` is the same JSON as a thread message's (`code[]`, `doc[]`,
`code_file_list`), filled by `JSON_SET` as the page run retrieves code and documents
([ee/tabby-db/src/pages.rs:20-29](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/pages.rs#L20-L29),
[:263-310](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/pages.rs#L263-L310)).
A page is generated from a thread's messages (`convertThreadToPage`) or from a title prompt the
user typed (`createPageRun`)
([ee/tabby-webserver/src/service/page.rs:92-133](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/page.rs#L92-L133),
[:135-150](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/page.rs#L135-L150)),
and is a row of its own, so it outlives the thread it came from; it is removed only by `delete_page`
([ee/tabby-db/src/pages.rs:123-124](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/pages.rs#L123-L124)).
It records no thread id, so it joins a thread on `author_id`, text and time only.

#### Editor command histories

Two editor clients keep the user's recent inline-edit instructions,
without times:

- IntelliJ: `intellij-tabby-command-history.xml`, state `CommandHistoryState.history`, a list of up
  to 30 strings, newest first
  ([clients/intellij/.../inlineChat/CommandHistory.kt:5-31](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/intellij/src/main/kotlin/com/tabbyml/intellijtabby/inlineChat/CommandHistory.kt#L5-L31)),
  stored under the component name `com.tabbyml.intellijtabby.inlineChat.CommandHistory`. The XML
  layout inside the file (a `<component name=...>` holding `<option name="history">` and a list of
  `<option value=...>` elements) is the IntelliJ platform's serialisation of a `BaseState` list, not
  something Tabby's source spells out. Where the IDE writes the file is in
  [IntelliJ plugin](#intellij-plugin).
- VS Code: `edit.recentlyCommand` in the extension's global state, the `TabbyML.vscode-tabby` row of
  `state.vscdb`, a list of JSON-encoded commands, newest first, capped by the `chatEdit.history`
  advanced setting (default 20). Each entry is `JSON.stringify` of `{command, context}`, `context`
  a list of `ChatEditFileContext` (`referrer`, `uri`, optional `range`), and the reader also accepts
  an entry that is a bare command string
  ([clients/tabby-agent/src/protocol.ts:434-449](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/protocol.ts#L434-L449);
  [quickPick.ts:322-341](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/inline-edit/quickPick.ts#L322-L341))
  ([clients/vscode/src/Config.ts:115-118](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/Config.ts#L115-L118),
  [:139-145](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/Config.ts#L139-L145),
  [clients/vscode/src/inline-edit/quickPick.ts:344-351](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/inline-edit/quickPick.ts#L344-L351)).
  The catalog attributes `state.vscdb` to `vscode`, so a Tabby reader of it needs `reads_agents`.

#### Source-host integrations

An administrator connects GitHub or GitLab to the server so that it can index repositories and
their issues and pull requests. `integrations(id, kind, display_name, access_token, api_base,
error, created_at, updated_at, synced)` holds each connection and `provided_repositories(id,
integration_id, vendor_id, name, git_url, active, created_at, updated_at, refs)` the repositories
listed from it
([ee/tabby-db/schema/schema.sql:142-165](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L142-L165)).
`kind` is `github`, `gitlab`, `github_self_hosted` or `gitlab_self_hosted`
([ee/tabby-schema/src/dao.rs:621-635](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/dao.rs#L621-L635)); the
two hosted kinds talk to `https://api.github.com` and `https://gitlab.com`, the self-hosted kinds to
their `api_base`
([ee/tabby-schema/src/schema/integration.rs:79-93](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/schema/integration.rs#L79-L93)).
The token is a personal access token typed into the server, stored in plain text and put into the
clone URL
([integration.rs:17-30](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/schema/integration.rs#L17-L30));
no column names the account it belongs to, only the administrator's `display_name`.

The repository list is a cache of what the token can reach: each sync fetches every repository the
token lists, upserts it with `updated_at = DATETIME('now')` and deletes the rows not seen in that
sync
([ee/tabby-webserver/src/service/repository/third_party.rs:173-201](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/repository/third_party.rs#L173-L201),
[:278-300](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/repository/third_party.rs#L278-L300);
[ee/tabby-db/src/provided_repositories.rs:22-38](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/provided_repositories.rs#L22-L38)).
`active` marks a repository the administrator chose to index
([provided_repositories.rs:119-138](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/provided_repositories.rs#L119-L138)).
Chat threads are not bound to a repository row: a message names the repository only through its
attachment's `git_url` ([thread_messages](#thread_messages)).

#### Pre-migration backups

The backups and the event log repeat what the live database records;
[doubleagent issue 122](https://github.com/seanthegeek/doubleagent/issues/122) is the rule that
parses both and drops the repeats.

`backup_db` copies the database file byte for byte, named `db.backup-<UTC date>.sqlite`
(`dev-db.backup-` for source builds), only when a migration is pending
([ee/tabby-db/src/lib.rs:189-221](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/lib.rs#L189-L221)).
A backup therefore holds the same thread ids, message texts, attachments and times as the live
database had on that day, in the schema of the release before the migration (a pre-0.25 backup keeps
its attachments in the old per-kind columns, which the parser renders the same way). It survives the
7-day deletion of ephemeral threads and the three-month pruning of `user_events`
([Event log against `user_events`](#event-log-against-user_events)); several
backups hold the same threads when nothing deleted them in between. They join the live database on
thread id (`session_id`) and on text and time exactly.

#### Event log against `user_events`

The server hands every event to two loggers through
`ComposedLogger`
([crates/tabby-common/src/api/event.rs:133-149](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/api/event.rs#L133-L149);
[ee/tabby-webserver/src/webserver.rs:45-46](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/webserver.rs#L45-L46)):
the file logger, which appends the serialised `LogEntry` to the day's file
([crates/tabby/src/services/event.rs:104-117](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/services/event.rs#L104-L117)),
and the database logger, which writes the same `entry.event`, pretty-printed, to `user_events` with
`entry.ts` as `created_at`
([ee/tabby-webserver/src/service/event_logger.rs:20-127](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/event_logger.rs#L20-L127)).
So a `user_events` row is the same event as a log line, with:

- `created_at` truncated to the second: the millisecond `ts` becomes a `%F %X` string
  ([ee/tabby-db/src/user_events.rs:20-42](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/user_events.rs#L20-L42);
  [lib.rs:307-310](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/lib.rs#L307-L310)),
  under a second from the log line;
- a row only when the log line's `user` decodes to a user row id; the `user` field is the id Tabby's
  GraphQL API (served with the juniper library) hands out, the row id encoded with Hashids (a
  reversible integer-to-string encoding, salt `tabby-id-serializer`, minimum length 6), and an event
  with no user is not stored
  ([event_logger.rs:16-18](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/event_logger.rs#L16-L18);
  [ee/tabby-schema/src/dao.rs:556-575](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/dao.rs#L556-L575));
- rows older than three months deleted by the database background job
  ([user_events.rs:77-95](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/user_events.rs#L77-L95);
  [ee/tabby-webserver/src/service/background_job/db.rs:88](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/background_job/db.rs#L88)),
  while the log has no retention.

The log is the fuller record, but the database is the store Tabby reads back: its user-event service
(`UserEventService::list`) queries `user_events`
([ee/tabby-webserver/src/service/user_event.rs:23-37](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/user_event.rs#L23-L37)),
and nothing but the writer opens the events directory
([crates/tabby-common/src/path.rs:52](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/path.rs#L52);
[event.rs:21](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/services/event.rs#L21)).
The analyzer also marks whole files, and the database file holds the threads as well, so the
database is primary and the log secondary. Nothing is lost by it: a log line with no `user_events`
twin (no user, a server run without the database, pruned after three months) is kept. The rows join
on `completion_id` (`session_id`), `segments.git_url` (`project_path`), the prompt and choice texts
and a time within a second. The `system` rows do not join: the parser names the owner by the log's
encoded id in one and by the account's email in the other, and the encoded id cannot be resolved to
an email without the database, so both copies of a completion's `system` row and of a view, select,
dismiss or chat-completion event are kept.

### Format versions

Migrations under `ee/tabby-db/migrations/`; `thread_messages` arrived in `0035`
([0035_add-thread-message-table.up.sql](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/migrations/0035_add-thread-message-table.up.sql)),
and the separate `code_attachments`, `client_code_attachments`, `doc_attachments` columns were
deprecated in 0.25 in favour of `attachment`
([threads.rs:34-38](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/src/threads.rs#L34-L38))
and are absent from the current schema. A backup from before 0.25 can carry attachments only in the
old columns. Event log lines have no version.

### Secrets

`~/.tabby/ee/db.sqlite` and its backups hold these credentials, plaintext unless noted
([ee/tabby-db/schema/schema.sql](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql)):
`users.auth_token` (each user's API token) and `password_encrypted` (hash)
([:17-30](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L17-L30)),
`registration_token.token`
([:9-15](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L9-L15)),
`email_setting.smtp_password`
([:72-82](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L72-L82)),
`oauth_credential.client_secret`
([:83-93](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L83-L93)),
`refresh_tokens.token`
([:126-134](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L126-L134)),
`integrations.access_token` (GitHub/GitLab tokens,
[:142-152](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L142-L152)),
`ldap_credential.bind_password`
([:249-266](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L249-L266)).
Redact those columns. Completion prompts and `clipboard` segments can carry pasted secrets.

### Parser plan

Globs: `.tabby/ee/db.sqlite`, `.tabby/ee/db.backup-*.sqlite`,
`.tabby/ee/dev-db.sqlite`, `.tabby/events/*.json`, `.tabby/config.toml`,
`JetBrains/*/options/intellij-tabby-command-history.xml` under the three IDE configuration roots,
and `User/globalStorage/state.vscdb` of VS Code and its forks (the parser's `reads_agents`). In a
Docker or Podman volume matching `*tabby*`, collected with its `_data` directory as the base, the
server root is the base itself, so the same files arrive without the leading `.tabby/`
([Server root `~/.tabby`](#server-root-tabby)); the parser takes both forms. The volume form is
source-derived; not observed on any real install.

| Column | Source |
| --- | --- |
| timestamp_utc | `thread_messages.created_at`; event log `ts` (ms) |
| session_id | `threads.id`; for completions `completion_id` |
| project_path | `attachment.code[0].git_url` or `segments.git_url`, else empty |
| git_branch | empty |
| turn_type | `role`; completion `prompt` → user, `choices[].text` → assistant; view/select/dismiss → system |
| model | empty (not recorded) |
| tool_name, tool_use_id | empty |
| text | `content`; completion `prompt` and choice text; others the event kind |

Add the thread owner's `users.email` to `text` or a note, since one
server serves many people.

#### Permission rows

The agent records none. Tabby's chat and answer engine run no tools on the host, so there is no
per-call decision or standing grant: `tool_calls` and `ToolCall` occur in `crates/` and `ee/` only
in a test helper (`ee/tabby-webserver/src/service/answer/testutils/mod.rs`), and `approv` matches no
Rust file.

#### Model rows

Tabby records no model in the thread or event stores, so the parser yields
no `model:` rows.

A thread run's `ThreadRunOptionsInput.model_name`
([ee/tabby-schema/src/schema/thread/inputs.rs:73-76](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-schema/src/schema/thread/inputs.rs#L73-L76))
is passed to the chat request
([ee/tabby-webserver/src/service/answer.rs:210](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-webserver/src/service/answer.rs#L210))
and not stored: `thread_messages` has no model column
([schema.sql:179-190](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L179-L190)).
The configured models are in `.tabby/config.toml`: `[model.completion]`, `[model.chat]` and
`[model.embedding]`, each `http` with `model_name` and `supported_models` or `local` with `model_id`
([crates/tabby-common/src/config.rs:259-263](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L259-L263),
[:302-342](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/config.rs#L302-L342)).
The parser yields one `system` row per configured model at the file's modification time, which
says which models the server could use, not which one answered:
`model configured: <group> <model_name or model_id> via <http|local>[ kind=<kind>][
endpoint=<api_endpoint>][ supported_models=<a,b>] (from config.toml [model.<group>.<http|local>],
file mtime)`, `<group>` being `completion`, `chat` or `embedding`; `api_key` never reaches a row.
The tables are read with Python's `tomllib`, so on Python 3.10 the parser yields one row
`parser: config.toml model tables not read: Python 3.11 or later needed` instead. These rows are
source-derived; not observed on any real install.

#### Past-use rows and secondary sources

Sources, primary or secondary (see [Pre-migration backups](#pre-migration-backups) and
[Event log against `user_events`](#event-log-against-user_events)):

| File | Role | Time rule |
| --- | --- | --- |
| `.tabby/ee/db.sqlite`, `dev-db.sqlite` | primary | own `created_at` |
| `.tabby/ee/db.backup-*.sqlite`, `dev-db.backup-*.sqlite` | secondary, one leading `system` row naming the file | own `created_at`, identical to the live copy; default 2-second window |
| `.tabby/events/YYYY-MM-DD.json` | secondary | own `ts` in milliseconds, under a second after `user_events.created_at`; default 2-second window |

Also read, source-derived; not observed on any real install:

| File | Role | Rows |
| --- | --- | --- |
| `pages`, `page_sections` in the database files ([Pages](#pages)) | primary in `db.sqlite`, secondary in a backup (the role is per file) | `page start \| title <title> \| author <email (name)> \| code_source <id>` as `system`, then the page `content` and each section (`<title>`, a blank line, `<content>`, the attachment summary) as `assistant`, in `position` order; `session_id` `page:<id>`, `project_path` the first `code[].git_url` of the sections, times the page's and each section's `created_at` |
| `intellij-tabby-command-history.xml` ([Editor command histories](#editor-command-histories)) | secondary | `inline edit command history: <N> commands, newest first (from intellij-tabby-command-history.xml, no times)`, then one `user` row per command, `source_line` its position, empty timestamp |
| `state.vscdb`, row `TabbyML.vscode-tabby`, key `edit.recentlyCommand` | secondary | the same leading row `(from state.vscdb TabbyML.vscode-tabby edit.recentlyCommand, no times)`, then one `user` row per command with ` [context: <uri>; ...]` appended when the entry names files |

A page has no thread id, so it does not pair with the thread it came from. The command histories
pair with nothing, since the server logs an inline edit only as an empty `chat_completion` event,
and are kept. Only the history key of the VS Code row is read: the row also holds the endpoint
tokens ([VS Code extension](#vs-code-extension)).

#### Sync, relay and origin rows

Tabby records no sync state, because it uploads no session to the
vendor; the server is the only store and the share feature stays on it.
A shared thread is marked by `threads.is_ephemeral = 0` (see
[Shared threads](#shared-threads)), which the `thread start` row
already carries by omission (it says `ephemeral` only for an unshared
thread); a row of the form `thread shared: visible to all users of this
server (from db.sqlite, threads.updated_at upper bound)` would make it
explicit, timestamped by nothing of its own, so empty, with the upper
bound in the text. What leaves the host besides model API calls is
usage telemetry, which carries no prompt, reply or thread text:

- The server posts a `ServeHealth` event every 3000 seconds to `https://app.tabbyml.com/api/usage`,
  with the anonymous UUID from `.tabby/usage_anonymous_id` as `distinct_id` and the `HealthState`
  (model names, device, CPU, version, webserver flag) as properties; on by default, off when
  `TABBY_DISABLE_USAGE_COLLECTION` is set
  ([crates/tabby-common/src/usage.rs:13-82](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby-common/src/usage.rs#L13-L82);
  [crates/tabby/src/serve.rs:362-377](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/serve.rs#L362-L377);
  [crates/tabby/src/services/health.rs:12-33](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/crates/tabby/src/services/health.rs#L12-L33)).
- `tabby-agent`, the editor clients' language server, posts `AgentInitialized`, `AgentConnected` and
  `AgentStats` (completion counts and latency percentiles) to the same `/usage` endpoint with the
  `anonymousId` from `.tabby-client/agent/data.json`
  ([clients/tabby-agent/src/telemetry.ts:40-121](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/telemetry.ts#L40-L121);
  [clients/tabby-agent/src/codeCompletion/statistics.ts:59-84](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/codeCompletion/statistics.ts#L59-L84)).
  On by default (`anonymousUsageTracking.disable` false,
  [clients/tabby-agent/src/config/default.ts:125-127](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/config/default.ts#L125-L127)),
  off through `[anonymousUsageTracking] disable = true` in the client `config.toml`, the editor
  setting, or the server's security setting `disable_client_side_telemetry`
  ([clients/tabby-agent/src/config/index.ts:57-63](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/config/index.ts#L57-L63)),
  stored in `server_setting.security_disable_client_side_telemetry`
  ([schema.sql:64](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-db/schema/schema.sql#L64)).
  The client logs each send only at trace level, and the log level defaults to `silent`
  ([default.ts:119-121](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/tabby-agent/src/config/default.ts#L119-L121)).

No client keeps a server thread id on disk. The VS Code chat webview keeps the panel's
`SessionState` (`threadId`, `qaPairs`, input, context) in an in-memory `Map` keyed by server
endpoint
([clients/vscode/src/chat/webview.ts:91-92](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/chat/webview.ts#L91-L92);
[:529-553](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/clients/vscode/src/chat/webview.ts#L529-L553);
[ee/tabby-ui/lib/types/chat.ts:228-234](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-ui/lib/types/chat.ts#L228-L234)),
and the web UI's persisted `tabby-chat-storage` local-storage entry holds the selected model and
repository but excludes `activeChatId`
([ee/tabby-ui/lib/stores/chat-store.ts:8-48](https://github.com/TabbyML/tabby/blob/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f/ee/tabby-ui/lib/stores/chat-store.ts#L8-L48)).
The IntelliJ plugin does not implement the session-state calls. Searched `share`, `sync`, `upload`,
`cloud`, `export`, `telemetry`, `usage`, `threadId`, `localStorage`, `globalState` and `https://` in
`crates/`, `ee/` and `clients/`.

#### Scratch directory rows

The `.tabby/repositories/` clones ([Scratch directories](#scratch-directories)) are excluded by the
catalog. The parser's `project_path` is a repository URL, never a local directory, so no scratch
directory can appear as a project.

#### Deletion rows

No deletion row. A pair deleted with `deleteThreadMessagePair` survives only
in a backup; the backups are already read. See [What survives a delete](#what-survives-a-delete).

#### Bad-line rows

The event log, the one line-oriented source, is read through `iter_jsonl` and its bad lines are
reported through `error_rows`, a log that cannot be read giving one whole-file row
`parser: <file> could not be read (<reason>)`, `<file>` its name; the SQLite stores have no lines
to lose. A `config.toml` that
cannot be read gives one row `parser: config.toml could not be read: <reason>` on every Python
version, one that does not parse one row `parser: config.toml is not valid TOML (<reason>)` and
an IntelliJ history that does not parse one row
`parser: not a readable XML document (<reason>)`, none carrying the file's text.

A `db.sqlite`, a backup copy beside it or a `state.vscdb` that cannot be read, or whose `-wal`,
`-shm` or `-journal` sidecar cannot, is one row `parser: <file> could not be read (<reason>)`,
`<file>` the name of the file that failed; one that is not a SQLite database is
`parser: not a SQLite database` for `db.sqlite` and its copies and
`parser: state.vscdb is not a SQLite database` for the editor state, and a damaged editor state is
`parser: state.vscdb unreadable: <message>`.

#### Fixtures

```sql
INSERT INTO users(id,email,is_admin,auth_token,active,name) VALUES(1,'alice@example.com',1,'auth_REDACT',1,'Alice');
INSERT INTO threads(id,is_ephemeral,user_id,created_at,updated_at,relevant_questions) VALUES(7,0,1,'2026-10-01 10:00:00','2026-10-01 10:00:09','["How is the cache invalidated?"]');
INSERT INTO thread_messages(id,thread_id,role,content,created_at,updated_at,attachment) VALUES
(1,7,'user','Where is the cache cleared?','2026-10-01 10:00:00','2026-10-01 10:00:00','{"code":null,"client_code":[{"filepath":"src/cache.rs","start_line":10,"content":"fn clear()"}],"doc":null}'),
(2,7,'assistant','In `clear()` in src/cache.rs.','2026-10-01 10:00:02','2026-10-01 10:00:09','{"code":[{"git_url":"https://github.com/acme/app","commit":"abc123","language":"rust","filepath":"src/cache.rs","content":"fn clear() {}","start_line":10}],"client_code":null,"doc":null}');
```

```json
{"user":"1","ts":1759312800123,"event":{"completion":{"completion_id":"cmpl-7f3a","language":"python","prompt":"def add(a, b):\n    ","segments":{"prefix":"def add(a, b):\n    ","suffix":"\n","git_url":"https://github.com/acme/app","filepath":"app/math.py"},"choices":[{"index":0,"text":"return a + b"}],"user_agent":"tabby-agent/1.9"}}}
{"user":"1","ts":1759312801500,"event":{"select":{"completion_id":"cmpl-7f3a","choice_index":0,"view_id":"view-1","elapsed":1377}}}
```

```sql
INSERT INTO pages(id,author_id,title,content,created_at,updated_at,code_source_id) VALUES(4,1,'Cache invalidation','How the cache is cleared.','2026-10-01 11:00:00','2026-10-01 11:00:30','acme-app');
INSERT INTO page_sections(id,page_id,title,content,position,created_at,attachment) VALUES(9,4,'Clearing','Call `clear()`.',0,'2026-10-01 11:00:10','{"code":[{"git_url":"https://github.com/acme/app","filepath":"src/cache.rs","start_line":10}]}');
INSERT INTO integrations(id,kind,display_name,access_token,created_at,updated_at) VALUES(1,'github','acme','ghp_REDACT','2026-09-01 09:00:00','2026-09-01 09:00:00');
INSERT INTO provided_repositories(id,integration_id,vendor_id,name,git_url,active,created_at,updated_at) VALUES(3,1,'101','acme/app','https://github.com/acme/app',1,'2026-09-01 09:05:00','2026-09-02 09:05:00');
```

```toml
[model.chat.http]
kind = "openai/chat"
model_name = "gpt-4o"
api_endpoint = "https://api.openai.com/v1"
api_key = "sk-REDACT"
supported_models = ["gpt-4o", "gpt-4o-mini"]

[model.completion.local]
model_id = "StarCoder-1B"
```

```xml
<application>
  <component name="com.tabbyml.intellijtabby.inlineChat.CommandHistory">
    <option name="history">
      <list>
        <option value="add docstrings" />
      </list>
    </option>
  </component>
</application>
```

```json
{"server.serverRecords":{"http://localhost:8080":{"token":"auth_REDACT"}},"edit.recentlyCommand":["{\"command\":\"rename @file to snake case\",\"context\":[{\"referrer\":\"@file\",\"uri\":\"file:///srv/proj/app/math.py\"}]}","fix the typo"]}
```

#### Source host rows

Tabby records a source-host integration server-side, in the database it already collects
([Source-host integrations](#source-host-integrations)): the connection's kind, its display name,
its API base and its token, and the cached list of repositories the token reaches. No column names
the connected account, and no thread is bound to a repository row. The token is a column of
`db.sqlite` and its backups, never selected ([Secrets](#secrets)); no editor client holds a
source-host token. The parser yields, at the record's own time and source-derived; not observed on
any real install:

- per integration, `source host: <host> integration <display_name> kind=<kind>
  api_base=<api_base or -> (account not recorded; from integrations table, created_at)`, `<host>`
  `github.com` or `gitlab.com` for the hosted kinds and the host of `api_base` for the self-hosted
  ones;
- per cached repository, `source host repository: <git_url> type=<kind> name=<name>
  active=<yes|no> (integration <display_name>; from provided_repositories table, updated_at)`.

The rows from a backup repeat the live database's and pair with them. The administrator's sign-in
through GitHub or GitLab OAuth (`oauth_credential`) is the server's own login for its users, not an
integration the agent acts through; it yields no row.

## Confidence

- High (source): the `~/.tabby` layout, env overrides, database file names, WAL, pre-migration
  backups, table and column names, role values, event file naming, fields, JSON shape and `ts`
  unit, the ephemeral cleanup, the `~/.tabby-client/agent` files, the VS Code `globalState` keys.
- Medium: Windows `home_dir()` resolving to the profile directory (the `home` crate's documented
  behaviour, not re-read here); the JetBrains `options/` location (vendor docs); the exact form of
  `user` in the event log (the Hashids-encoded user row id the database logger decodes, see
  [Event log against `user_events`](#event-log-against-user_events); how a request's user reaches
  the entry was not traced through the auth middleware); the XML layout of
  `intellij-tabby-command-history.xml` ([Editor command histories](#editor-command-histories)).
- Not determined: Eclipse plugin storage; the size of `events/` on a busy server (no retention in
  the writer, so it grows without bound); whether the IntelliJ command history is stored in
  `options/`, where the catalog collects it, or the project's `.idea/`, which no catalog line
  reaches ([IntelliJ plugin](#intellij-plugin)).
