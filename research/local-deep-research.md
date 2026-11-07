# local-deep-research

Local Deep Research (LDR) is a research agent with a web UI, written in Python. Its parser is in
the backlog ([issue 56](https://github.com/seanthegeek/doubleagent/issues/56)): research history
and chat live in a per-user SQLCipher (encrypted SQLite) database encrypted with the user's
login password ([Credentials](#credentials)), so the analyzer cannot parse them without that
password.

## Source and evidence

[LearningCircuit/local-deep-research](https://github.com/LearningCircuit/local-deep-research) (LDR),
MIT, open source, Python, commit
[`3b965431351bb17c523fa38fdcfced4a3f891d49`](https://github.com/LearningCircuit/local-deep-research/commit/3b965431351bb17c523fa38fdcfced4a3f891d49).
All claims are from source, apart from the pre-1.0 layout, which comes from the repository's own
docs. Platform directory resolution was checked in
[tox-dev/platformdirs](https://github.com/tox-dev/platformdirs) at
[`960ff632eb575deb6f0d5bf49519d5a2ee2098b6`](https://github.com/tox-dev/platformdirs/commit/960ff632eb575deb6f0d5bf49519d5a2ee2098b6).
LDR pins `platformdirs~=4.5`
([pyproject.toml:62](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/pyproject.toml#L62)),
and that checkout is newer.

### Observed

- No real install: the evidence is source and the repository's docs.

### Source reviews

- 2026-10-19: HEAD
  [`3c9eea9`](https://github.com/LearningCircuit/local-deep-research/commit/3c9eea9ac132a0077ca5918140e94f4d56868c6a)
  against the previously cited
  [`3b96543`](https://github.com/LearningCircuit/local-deep-research/commit/3b965431351bb17c523fa38fdcfced4a3f891d49),
  from source only. The paths were unchanged; it added the model, backup, deletion and
  notification records inside the encrypted database ([Database detail](#database-detail)).

## Collection

### Per-user storage

The data directory is `$LDR_DATA_DIR` if set (absolute path required), otherwise
`platformdirs.user_data_dir("local-deep-research")`
([config/paths.py:28-61](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/config/paths.py#L28-L61);
the same paths at [`3c9eea9`](https://github.com/LearningCircuit/local-deep-research/commit/3c9eea9ac132a0077ca5918140e94f4d56868c6a), where `paths.py` was reorganised and the resolution is at
[paths.py:16-63](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/config/paths.py#L16-L63)):

- Linux: `$XDG_DATA_HOME/local-deep-research`, else `~/.local/share/local-deep-research`
  ([platformdirs unix.py:40-42](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/unix.py#L40-L42)).
- macOS: `~/Library/Application Support/local-deep-research`
  ([macos.py:22-23](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/macos.py#L22-L23),
  [31-33](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/macos.py#L31-L33)).
  Newer platformdirs lets `XDG_*` override this on macOS
  ([macos.py:171-179](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/macos.py#L171-L179)).
- Windows: `%LOCALAPPDATA%\local-deep-research\local-deep-research`. With no `appauthor`,
  platformdirs repeats the app name as the author
  ([windows.py:32-51](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/windows.py#L32-L51)).
  The comment in LDR's `paths.py:52` omits the doubled segment.

Contents, all created on demand
([paths.py:64-213](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/config/paths.py#L64-L213)):

- `ldr_auth.db`: plain SQLite, table `users` (`username`, `created_at`, `last_login`,
  `database_version`)
  ([database/auth_db.py:1-4](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/auth_db.py#L1-L4),
  [33-35](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/auth_db.py#L33-L35),
  [models/auth.py:32-40](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/models/auth.py#L32-L40)).
  This is the list of LDR accounts on the host.
- `encrypted_databases/ldr_user_<sha256(username)[:16]>.db`, with a `.salt` sidecar and WAL files:
  the per-user SQLCipher database
  ([paths.py:135-158](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/config/paths.py#L135-L158),
  [encrypted_db.py:384-390](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/encrypted_db.py#L384-L390),
  [564-575](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/encrypted_db.py#L564-L575),
  [sqlcipher_utils.py:27-28](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/sqlcipher_utils.py#L27-L28),
  [656-666](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/sqlcipher_utils.py#L656-L666)).
  Tables include `research_history` (`query`, `mode`, `status`, `created_at`, `report_content`,
  `progress_log`, …;
  [models/research.py:310-343](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/models/research.py#L310-L343)),
  `chat_sessions`, `chat_messages`
  ([models/chat.py:59-65](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/models/chat.py#L59-L65),
  [140-147](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/models/chat.py#L140-L147)),
  `settings` and `api_keys`.
- `encrypted_databases/backups/<user hash>/ldr_backup_*.db`: encrypted backups
  ([paths.py:211-242](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/config/paths.py#L211-L242),
  [backup_service.py:249](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/backup/backup_service.py#L249)).
- `research_outputs/<research_id>.md` and `<research_id>_metadata.json`: plaintext report copies
  in a directory created with mode 0700 since
  [`3c9eea9`](https://github.com/LearningCircuit/local-deep-research/commit/3c9eea9ac132a0077ca5918140e94f4d56868c6a),
  written only when `report.enable_file_backup` is on (default `false`)
  ([storage/file.py:32-84](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/storage/file.py#L32-L84),
  [storage/factory.py:36-45](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/storage/factory.py#L36-L45),
  [default_settings.json:925-937](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/defaults/default_settings.json#L925-L937)).
- `logs/<name>.log`: plaintext, only when `LDR_ENABLE_FILE_LOGGING=true`; 10 MB rotation, 7-day
  retention, zip-compressed
  ([log_utils.py:1095-1110](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/utilities/log_utils.py#L1095-L1110)).
  Messages pass a credential redactor first
  ([log_utils.py:805-812](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/utilities/log_utils.py#L805-L812)).
- `config/config.toml`: written from the web UI
  ([web/routers/research.py:2550-2560](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/web/routers/research.py#L2550-L2560)).
  `server_config.json` is legacy
  ([web/server_config.py:84-86](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/web/server_config.py#L84-L86)).
- `.secret_key`: web session signing key, mode 0600
  ([web/fastapi_app.py:176-188](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/web/fastapi_app.py#L176-L188)).
- `cache/` including `cache/rag_indices/<user hash>/` vector indexes
  ([library_rag_service.py:679-686](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/research_library/services/library_rag_service.py#L679-L686)),
  `models/` (embedding models), `journal_data/` (OpenAlex and DOAJ downloads,
  [paths.py:93-103](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/config/paths.py#L93-L103)),
  `library/`, `benchmark_results/`
  ([benchmark_commands.py:57](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/benchmarks/cli/benchmark_commands.py#L57)).

#### Scratch directories

None. LDR creates no working directory for the model; `research_outputs/` and the download library
([Document library](#document-library)) are its own output stores. `research_outputs/` is
collected with the data directory; the library is under the collected
`Documents/LocalDeepResearch` but excluded ([Exclusions](#exclusions)).

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `LDR_DATA_DIR` | the whole data directory, an absolute path ([paths.py:28-47](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/config/paths.py#L28-L47)) | `.local/share/local-deep-research`, `Library/Application Support/local-deep-research`, `AppData/Local/local-deep-research/local-deep-research` | `local-deep-research` |
| `XDG_DATA_HOME` | the parent of the data directory on Linux ([platformdirs unix.py:40-42](https://github.com/tox-dev/platformdirs/blob/960ff632eb575deb6f0d5bf49519d5a2ee2098b6/src/platformdirs/unix.py#L40-L42)) | `.local/share` | `shared` |

Not relocation variables: `LDR_ENABLE_FILE_LOGGING` (behaviour only), `SENTENCE_TRANSFORMERS_HOME`
(moves only the excluded embedding-model cache).

#### Document library

The downloaded-document library defaults to `~/Documents/LocalDeepResearch/Library`, outside the
data directory, on every OS
([library_settings.json:2-13](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/defaults/research_library/library_settings.json#L2-L13),
[library_service.py:1212-1224](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/research_library/services/library_service.py#L1212-L1224)).

#### Pre-1.0 layout

Pre-1.0 (v0.x) kept a single unencrypted `ldr.db` in the data directory
([MIGRATION_GUIDE_v1.md:70-80](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/docs/MIGRATION_GUIDE_v1.md#L70-L80),
[env_configuration.md:244-248](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/docs/env_configuration.md#L244-L248)).

#### Docker

The Docker deployment sets `LDR_DATA_DIR=/data` on a named volume
([docker-compose.yml:57](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/docker-compose.yml#L57),
[127-128](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/docker-compose.yml#L127-L128)),
which is outside any home directory. Since collector 1.5.0 the `DOCKER_VOLUMES` table collects
`*ldr_data` volumes, with data-relative exclusions.

#### Database detail

The records inside the encrypted user database, at
[`3c9eea9`](https://github.com/LearningCircuit/local-deep-research/commit/3c9eea9ac132a0077ca5918140e94f4d56868c6a),
readable only with the user's password:

- **Permissions.** None on disk. LDR runs no host tools that ask for approval; operator approvals
  of private search-engine origins come from environment variables, not a file
  ([env_definitions/security.py:104](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/settings/env_definitions/security.py#L104)).
  No stored decision exists (grepped for `approv` and `permission` in `database/`: no hit in
  `database/models`, and the other hits are file-mode hardening).

- **Model.** `research_history.research_meta` keeps `submission` with `model_provider` and `model`
  for each research
  ([web/routers/research.py:1356-1360](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/web/routers/research.py#L1356-L1360),
  [1388-1390](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/web/routers/research.py#L1388-L1390),
  [1494](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/web/routers/research.py#L1494));
  `token_usage` (`research_id`, `timestamp`, `model_provider`, `model_name`, token counts, costs)
  records each call, and `model_usage` the totals per model
  ([models/metrics.py:20-45](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/database/models/metrics.py#L20-L45),
  [106-115](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/database/models/metrics.py#L106-L115)).
  The user's default is the `llm.provider` and `llm.model` settings
  ([default_settings.json:568](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/defaults/default_settings.json#L568),
  [681](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/defaults/default_settings.json#L681)).
- **Backups.** `backup.enabled` is on by default, with `backup.max_count` 1 and
  `backup.max_age_days` 7
  ([default_settings.json:5410-5450](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/defaults/default_settings.json#L5410-L5450));
  a backup is taken at login
  ([web/routers/auth.py:566](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/web/routers/auth.py#L566))
  and older ones are pruned
  ([backup_service.py:258-274](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/database/backup/backup_service.py#L258-L274),
  [865](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/database/backup/backup_service.py#L865)).
  The backup is the whole database as it was at the last login, so it still holds a research or chat
  deleted since, under the same password.
- **Deletion.** Deleting a research deletes it and every table that references it in one
  transaction, in `database/research_cascade.py`, a module new since
  [`3b96543`](https://github.com/LearningCircuit/local-deep-research/commit/3b965431351bb17c523fa38fdcfced4a3f891d49)
  ([research_cascade.py:499-530](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/database/research_cascade.py#L499-L530)),
  then unlinks the report file named by `report_path` when it lies inside the reports root
  ([web/routers/research.py:1919-1931](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/web/routers/research.py#L1919-L1931),
  [2090-2092](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/web/routers/research.py#L2090-L2092)).
  With file backup on, the storage layer also removes `<id>.md` and `<id>_metadata.json`
  ([storage/file.py:209-224](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/storage/file.py#L209-L224)).
- **Notifications.** `notifications.enabled`, `notifications.service_url` (one or more URLs for
  Apprise, a library that posts to Discord, Telegram, email and other services; the URLs embed the
  service's credentials) and per-event switches such as
  `notifications.on_research_completed` are settings in the encrypted database
  ([default_settings.json:5072-5184](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/defaults/default_settings.json#L5072-L5184)).
  The completion message carries the query, a summary and a link to the result
  ([research_completed.jinja2:1-7](https://github.com/LearningCircuit/local-deep-research/blob/3c9eea9ac132a0077ca5918140e94f4d56868c6a/src/local_deep_research/notifications/templates/research_completed.jinja2#L1-L7)),
  so a copy of each research's question and summary sits in the destination service, outside the
  host.

### Credentials

- Provider API keys are kept in the `api_keys` and `settings` tables inside the encrypted user
  database
  ([models/settings.py:43-58](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/models/settings.py#L43-L58)).
  No keychain is used. LDR reads server environment variables, not a `.env` file
  ([error_handling/report_generator.py:396](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/error_handling/report_generator.py#L396)).
- Encryption: SQLCipher, key derived with PBKDF2-HMAC-SHA512 (256,000 iterations) from the login
  password and the 32-byte `.salt` file; a missing salt file means the legacy fixed salt
  `b"no salt"`
  ([sqlcipher_utils.py:44-76](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/sqlcipher_utils.py#L44-L76),
  [355-358](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/sqlcipher_utils.py#L355-L358),
  [399](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/sqlcipher_utils.py#L399)).
  With `bootstrap.allow_unencrypted` the database is plain SQLite
  ([encrypted_db.py:522-528](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/encrypted_db.py#L522-L528)),
  and the keys are then readable. Passwords are held only in memory
  ([session_passwords.py:1-12](https://github.com/LearningCircuit/local-deep-research/blob/3b965431351bb17c523fa38fdcfced4a3f891d49/src/local_deep_research/database/session_passwords.py#L1-L12)).
- `.secret_key`: secret, since it can forge web sessions.
- `config/config.toml` may embed keys if the operator put them there (not verified).
- The pre-1.0 `ldr.db` is unencrypted, so it holds settings and keys in plaintext. Collect it
  unflagged as evidence.

### Exclusions

`cache`, `models`, `journal_data` and `library` under the data directory, and
`Documents/LocalDeepResearch/Library`. Embedding models, vector indexes, journal datasets and
downloaded PDFs are large and are not agent state. Each excluded directory is one manifest row
with its total size, not a list of its files
([`collectors/docs/output.md`](../collectors/docs/output.md)). Keep `encrypted_databases/backups`:
the files are encrypted copies, but they show when the database existed.

### Project-local files

None. LDR is not a coding agent and does not write into repositories.

### Where the project path is recorded

Nowhere. LDR has no project concept. The research query (`research_history.query`) is inside the
encrypted database.

## Parsing

LDR is `backlog`: its parser is
[issue 56](https://github.com/seanthegeek/doubleagent/issues/56), and nothing it writes is parsed
yet. The history and chat are in the per-user SQLCipher database, whose key derives from the login
password ([Credentials](#credentials)), so a parser needs that password, a database kept plain
with `bootstrap.allow_unencrypted`, or the unencrypted pre-1.0 `ldr.db`. The tables, the login
backups and the plaintext copies are in [Per-user storage](#per-user-storage) and
[Database detail](#database-detail). The opt-in log files are the only line-oriented files, and
they are zip-compressed on rotation.

## Confidence

- High: data directory resolution on all three OSes, every file name, the encryption scheme and
  the key storage (source).
- High: the Windows doubled `local-deep-research` segment, from platformdirs `_append_parts` (the
  pinned 4.5 line was not checked separately).
- Medium: the pre-1.0 `ldr.db` location (docs only, no older tag read).
- Not determined: the contents of `config/config.toml`, and whether `Documents` is redirected
  (OneDrive) on Windows, which would move the library.
