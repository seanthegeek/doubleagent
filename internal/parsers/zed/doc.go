// Package zed reads the Zed editor's agent panel threads, its sidebar
// database and the standing permission entries of its user settings files.
//
// Paths are under the data directory (.local/share/zed, Library/Application
// Support/Zed, AppData/Local/Zed or the Flatpak data/zed) or the config
// directory (.config/zed, AppData/Roaming/Zed or the Flatpak config/zed):
//
//   - threads/threads.db, table threads (id, summary, updated_at, data_type,
//     data, parent_id, folder_paths, created_at): data is the thread as JSON,
//     zstd-compressed when data_type is "zstd". Its messages are externally
//     tagged User, Agent, Resume or Compaction; the older role, segments,
//     tool_uses and tool_results shape is read too.
//   - db/0-<channel>/db.sqlite: sidebar_threads, trusted_worktrees, the
//     agent_draft_prompts and created_git_worktrees namespaces of
//     scoped_kv_store, archived_git_worktrees with thread_archived_worktrees,
//     and sidebar_terminal_threads, each as system rows.
//   - settings.json and global_settings.json (JSON with comments): the
//     standing permission keys, in every override block.
//
// Messages carry no time: each row takes the thread's updated_at, and the
// thread-start, `git remote:` and `model:` rows the project snapshot's time.
// Denials are read from failed tool results' texts; an approval leaves no
// record. The source line is the SQLite rowid. See research/zed.md.
package zed
