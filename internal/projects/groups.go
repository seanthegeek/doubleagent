package projects

// The six source groups of collectors/docs/internals.md, "Project
// discovery", in the sh collector's order: the order paths are first seen
// sets the project order, so each source keeps its place in its group.

// claudeCodeSources is group 1: Claude Code prompt history and the project
// keys of .claude.json, also in CLAUDE_CONFIG_DIR homes.
func (s *homeScan) claudeCodeSources() {
	s.jsonValues("project", []string{s.home + "/.claude/history.jsonl"})
	s.jsonPathKeys(s.home+"/.claude.json", `\{`)
	s.jsonValues("project", s.expand(".claude-*/history.jsonl"))
	for _, config := range s.expand(".claude-*/.claude.json") {
		s.jsonPathKeys(config, `\{`)
	}
}

// cliSessionSources is group 2: the CLI agents' session files. A Codex cwd
// in the desktop app's Documents/Codex or in a Codex worktree is no project.
func (s *homeScan) cliSessionSources() {
	var codex []string
	for _, dir := range []string{s.home + "/.codex/sessions", s.home + "/.codex/archived_sessions"} {
		codex = append(codex, s.sessionCwds(dir, "*.jsonl")...)
	}
	s.emitOutside(codex, []string{s.home + "/Documents/Codex"},
		[]string{s.home + "/Documents/Codex/", s.home + "/.codex/worktrees/"})
	s.jsonValues("cwd", s.expand(".qwen/projects/*/chats/*.jsonl", ".cline/data/sessions/*/*.json",
		".cursor/chats/*/*/meta.json"))
	s.jsonValues("workspace_root", s.expand(".cline/data/sessions/*/*.json"))
	s.jsonValues("workspaceDirectory", []string{s.home + "/.continue/sessions/sessions.json"})
	s.jsonValues("working_dir", s.expand(".local/share/goose/sessions/*.jsonl",
		"AppData/Roaming/Block/goose/data/sessions/*.jsonl"))
	s.jsonValues("worktree", s.expand(".local/share/opencode/storage/project/*.json",
		".local/share/kilo/storage/project/*.json"))
	s.jsonValues("canonical", s.expand(".local/share/kilo/memory/*/manifest.json"))
	s.jsonValues("path", []string{s.home + "/.local/share/crush/projects.json",
		s.home + "/AppData/Local/crush/projects.json"})
	chats := s.expand("agent-zero/usr/chats/*/chat.json", "agent-zero/*/usr/chats/*/chat.json",
		"Desktop/agent-zero/usr/chats/*/chat.json")
	s.jsonValues("acp_cwd", chats)
	// The sh grep for the extra directories reads the chats directly, so
	// readable does not filter them.
	s.emit(grepFiles(chats, additionalDirectoryMatcher()))
}

// geminiSources is group 3: Gemini CLI projects, trusted folders and
// project roots, then Antigravity CLI history, settings and caches.
func (s *homeScan) geminiSources() {
	s.jsonPathKeys(s.home+"/.gemini/projects.json", `"`)
	s.jsonPathKeys(s.home+"/.gemini/trustedFolders.json", `"`)
	s.catFiles(s.expand(".gemini/tmp/*/.project_root", ".gemini/history/*/.project_root"))
	s.jsonValues("workspace", []string{s.home + "/.gemini/antigravity-cli/history.jsonl"})
	s.emit(s.absStrings(append([]string{s.home + "/.gemini/antigravity-cli/settings.json",
		s.home + "/.gemini/antigravity-cli/cache/projects.json"},
		s.expand(".gemini/config/projects/*.json")...)))
}

// gooseSources is find_goose_projects, part of group 4: the desktop recent
// directories, scheduled jobs' recipe directories and the workingDirs list
// of each project file.
func (s *homeScan) gooseSources() {
	s.emit(s.absStrings([]string{s.home + "/.config/Goose/recent-dirs.json",
		s.home + "/Library/Application Support/Goose/recent-dirs.json",
		s.home + "/AppData/Roaming/Goose/recent-dirs.json"}))
	s.jsonValues("recipe_base_dir", []string{s.home + "/.local/share/goose/schedule.json",
		s.home + "/AppData/Roaming/Block/goose/data/schedule.json"})
	for _, project := range s.expand(".local/share/goose/projects/*.md",
		"AppData/Roaming/Block/goose/data/projects/*.md") {
		if isRegularFile(project) {
			s.emit(gooseWorkingDirs(lines(project)))
		}
	}
}

// agentSessionSources is group 4, first half: Amp, Factory, Kiro, Cursor ACP
// sessions, Goose, Copilot CLI, Hermes and Letta. A Copilot cwd under
// .copilot, or one of a session without events.jsonl, is no project.
func (s *homeScan) agentSessionSources() {
	s.jsonValues("cwd", append([]string{s.home + "/.local/share/amp/history.jsonl"},
		s.expand(".factory/sessions/*/*.jsonl", ".factory/sessions/*.jsonl",
			".kiro/sessions/cli/*.json", ".cursor/acp-sessions/*/meta.json")...))
	s.jsonValues("root", s.expand(".kiro/workspace-roots/*/.trust-migration.json"))
	s.emit(s.absStrings(s.expand(".kiro/sessions/*/sess_*/session.json")))
	s.gooseSources()
	var copilot []string
	for _, events := range s.expand(".copilot/session-state/*/events.jsonl") {
		if isRegularFile(events) {
			workspace := events[:len(events)-len("events.jsonl")] + "workspace.yaml"
			copilot = append(copilot,
				grepFiles(s.readable([]string{workspace}), yamlValueMatcher("cwd"))...)
		}
	}
	s.emitOutside(copilot, nil, []string{s.home + "/.copilot/"})
	s.jsonValues("workdir", s.expand(".hermes/checkpoints/store/projects/*.json",
		".hermes/profiles/*/checkpoints/store/projects/*.json",
		"AppData/Local/hermes/checkpoints/store/projects/*.json",
		"AppData/Local/hermes/profiles/*/checkpoints/store/projects/*.json"))
	s.jsonValues("project", []string{s.home + "/.letta/sessions.jsonl"})
	s.moreAgentSessionSources()
}

// moreAgentSessionSources is group 4, second half: Letta conversations, pi,
// muse, Open Interpreter, OpenHands and PearAI. An Open Interpreter cwd in
// one of its worktrees is no project.
func (s *homeScan) moreAgentSessionSources() {
	s.jsonValues("cwd", s.expand(".letta/lc-local-backend/conversations/*/messages.jsonl",
		".pi/agent/sessions/*/*.jsonl", ".pi/agent/*.jsonl", ".pi/agent/crashes.json",
		".pi/agent/experimental/sessions/*/meta.json"))
	s.jsonPathKeys(s.home+"/.pi/agent/trust.json", `[tf]`)
	s.jsonPathKeys(s.home+"/.config/muse/trust.json", `\{`)
	s.jsonValues("workspace_root", s.expand(".local/share/muse/sessions/*/*/*/*/session.jsonl"))
	var interpreter []string
	for _, dir := range []string{s.home + "/.openinterpreter/sessions",
		s.home + "/.openinterpreter/archived_sessions"} {
		interpreter = append(interpreter, s.sessionCwds(dir, "rollout-*.jsonl")...)
	}
	s.emitOutside(interpreter, nil, []string{s.home + "/.openinterpreter/worktrees/"})
	s.jsonValues("path", []string{s.home + "/.openhands/workspaces.json"})
	s.jsonValues("working_dir", s.expand(".openhands/agent-canvas/dev_conversations/*/meta.json",
		".openhands/agent-canvas/conversations/*/meta.json",
		".openhands/conversations/*/base_state.json"))
	s.jsonValues("workspaceDirectory", []string{s.home + "/.pearai/sessions/sessions.json"})
}

// configFileSources is group 5: OpenClaw and its Clawdbot and Moltbot
// configs, nanobot, Tabby and Twinny, whose manifest lists indexed files,
// so the project is the deepest directory they share.
func (s *homeScan) configFileSources() {
	for _, key := range []string{"workspace", "cwd", "agentDir"} {
		configs := append(s.expand(".openclaw*/openclaw.json"), s.home+"/.clawdbot/openclaw.json",
			s.home+"/.clawdbot/clawdbot.json", s.home+"/.clawdbot/moltbot.json",
			s.home+"/.moltbot/moltbot.json")
		s.emit(grepFiles(s.readable(configs), json5ValueMatcher(key)))
	}
	for _, file := range s.expand(".nanobot*/sessions/*/.workspace") {
		if isRegularFile(file) {
			s.emit(lines(file))
		}
	}
	s.jsonValues("workspace", s.expand(".nanobot*/config.json"))
	s.emit(grepFiles(s.readable([]string{s.home + "/.tabby/config.toml"}),
		tomlFileURLMatcher("git_url")))
	for _, manifest := range s.expand(".twinny/embeddings/*/manifest.json") {
		if !isRegularFile(manifest) {
			continue
		}
		if dir, ok := commonDir(s.absStrings([]string{manifest})); ok {
			s.emit([]string{dir})
		}
	}
}

// editorSources is group 6: each VS Code-family User directory's
// workspaceStorage folders and the Cline, Roo Code and Kilo Code task
// indexes.
func (s *homeScan) editorSources() {
	for _, userDir := range s.expand("Library/Application Support/*/User", ".config/*/User",
		"AppData/Roaming/*/User", ".*-server*/data/User") {
		if !isDirectory(userDir) {
			continue
		}
		if storage := userDir + "/workspaceStorage"; isDirectory(storage) {
			s.emit(grepFiles(findNamed(s.ctx, storage, "workspace.json"), workspaceFolderMatcher()))
		}
		global := userDir + "/globalStorage"
		s.jsonValues("cwdOnTaskInitialization", expandIn(global,
			"saoudrizwan.claude-dev/state/taskHistory.json",
			"saoudrizwan.claude-dev/tasks/*/task_metadata.json"))
		s.jsonValues("workspace", expandIn(global, "rooveterinaryinc.roo-cline/tasks/_index.json",
			"kilocode.kilo-code/tasks/_index.json"))
	}
}
