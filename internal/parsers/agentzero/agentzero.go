// Package agentzero reads Agent Zero's chats, scheduler tasks, plugin
// settings and connector state.
//
// It reads <a0>/usr/chats/<ctxid>/chat.json with its messages/<n>.txt tool
// outputs (and the older <a0>/usr/chats/<ctxid>.json and <a0>/tmp/chats/
// copies), where <a0> is [Desktop/]agent-zero[/<instance>], or the same
// files under chats/ at the base of a collected /a0/usr Docker or Podman
// volume. Beside them it reads usr/scheduler/tasks.json and the Telegram
// plugin's state.json (both secondary sources), the _tool_access and
// _orchestrator plugin config.json and .toggle-0/.toggle-1 files, and the A0
// CLI connector's ~/.agent-zero/.env. It yields the chat log as user,
// assistant, thinking, tool_use, tool_result and system rows, and model,
// permission, sync, relay, origin and scheduled-task rows.
//
// Fields read are those listed in research/agent-zero.md; a task's token and
// system prompt are never read. A Telegram bot's token and webhook secret,
// a model configuration's API key and the connector .env's restore token,
// the one credential that file holds, are shown through parserkit.SecretText;
// the .env's other values are shown as written.
package agentzero

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// tasksTolerance is the window of the scheduler's rows: a task's last_run is
// stamped when the run finishes, after the chat's response log item, so its
// result is matched in a wider window.
const tasksTolerance = 60.0

// Parser reads Agent Zero's files.
type Parser struct {
	parserkit.Base
	chat, legacy, tasks, telegram, policy, toggle, connector *regexp.Regexp
	blocked, icon, logs                                      *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	// A home install keeps the chats under <a0>/usr/chats (or the legacy
	// <a0>/tmp/chats); a volume mounted at /a0/usr is collected with its _data
	// directory as the home, so its chats start at chats/.
	const chats = `(?:(?:Desktop/)?agent-zero/(?:[^/]+/)?(?:usr|tmp)/)?chats`
	// The other usr/ files read beside the chats, in the same two forms.
	const usr = `\A(?:(?:Desktop/)?agent-zero/(?:[^/]+/)?usr/)?`
	// A plugin's scoped directory: usr/plugins/<plugin>, a profile's
	// usr/agents/<profile>/plugins/<plugin>, a project's
	// usr/projects/<name>/.a0proj/plugins/<plugin> and a project profile's
	// .a0proj/agents/<profile>/plugins/<plugin>.
	const scope = `(?:projects/(?P<project>[^/]+)/\.a0proj/)?(?:agents/(?P<profile>[^/]+)/)?plugins/`
	return &Parser{
		Base:   parserkit.NewBase("agent-zero", "agent-zero"),
		chat:   regexp.MustCompile(`\A` + chats + `/[^/]+/chat\.json\z`),
		legacy: regexp.MustCompile(`\A` + chats + `/[^/]+\.json\z`),
		tasks: regexp.MustCompile(
			`\A(?:(?:Desktop/)?agent-zero/(?:[^/]+/)?(?:usr|tmp)/)?scheduler/tasks\.json\z`),
		telegram: regexp.MustCompile(usr + `plugins/_telegram_integration/state\.json\z`),
		policy:   regexp.MustCompile(usr + scope + `(?P<plugin>_tool_access|_orchestrator)/config\.json\z`),
		toggle: regexp.MustCompile(usr + scope +
			`(?P<plugin>_infection_check|_orchestrator)/\.toggle-(?P<state>[01])\z`),
		connector: regexp.MustCompile(`\A\.agent-zero/\.env\z`),
		// The message ensure_tool_allowed raises for a blocked tool, inside the
		// formatted error of the warning log item.
		blocked: regexp.MustCompile(
			`Tool "([^"]*)" is blocked for agent profile "([^"]*)"\.`),
		// A heading's icon marker, icon://<name>, runs to the next whitespace
		// (unicode.IsSpace) and is removed with the whitespace after it.
		icon: regexp.MustCompile(`icon://[^` + parserkit.Space + `]+[` + parserkit.Space + `]*`),
		logs: regexp.MustCompile(`"logs"[ \t\n\r]*:[ \t\n\r]*\[`),
	}
}

// Marks says the parser records synced, relayed and remote-origin sessions.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Relay: true, Origin: true}
}

// Wants reports whether the artifact is a chat, scheduler, Telegram, plugin
// or connector file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the chat, scheduler, Telegram, plugin and connector files.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("chat.json", p.chat),
		parserkit.PatternKind("legacy chat", p.legacy), parserkit.PatternKind("scheduler tasks.json", p.tasks),
		parserkit.PatternKind("Telegram state.json", p.telegram), parserkit.PatternKind("plugin config.json", p.policy),
		parserkit.PatternKind("plugin toggle", p.toggle), parserkit.PatternKind("connector .env", p.connector)}
}

// Secondary reports whether the artifact is the scheduler's tasks or the
// Telegram chat map.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.tasks.MatchString(artifact.Rel) || p.telegram.MatchString(artifact.Rel)
}

// SecondaryTolerance returns the wider window of the scheduler, whose
// last_run follows the reply.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if p.tasks.MatchString(artifact.Rel) {
		return tasksTolerance
	}
	return parserkit.DefaultTolerance
}

// Parse emits the rows of one Agent Zero file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case p.tasks.MatchString(rel):
		p.parseTasks(artifact, emit)
	case p.telegram.MatchString(rel):
		return p.parseTelegram(artifact, emit)
	case p.policy.MatchString(rel):
		p.parsePolicy(artifact, p.policy.FindStringSubmatch(rel), emit)
	case p.toggle.MatchString(rel):
		p.parseToggle(artifact, p.toggle.FindStringSubmatch(rel), emit)
	case p.connector.MatchString(rel):
		p.parseConnector(artifact, emit)
	default:
		return p.parseChat(artifact, options, emit)
	}
	return nil
}

// groupOf returns the text a named group of a match holds.
func groupOf(pattern *regexp.Regexp, match []string, name string) string {
	return match[pattern.SubexpIndex(name)]
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
