// Package kilocode reads Kilo Code's sessions and settings.
//
// Kilo Code's CLI, terminal UI and server are a fork of OpenCode with the
// same SQLite and JSON stores, so the parser holds an OpenCode parser made
// for Kilo's directories: .local/share/kilo/kilo.db, kilo-<channel>.db and
// opencode-<channel>.db, the storage/ and project/<slug>/storage/ JSON
// trees, and .local/state/kilo's prompt-history.jsonl and session.json
// (both secondary) and prompt-stash.jsonl. Every database is primary. It
// also reads Kilo's own files:
//
//   - the pre-fork editor extension's tasks,
//     <editor user directory>/User/globalStorage/kilocode.kilo-code/tasks/
//     <id>/api_conversation_history.json, a JSON array of chat messages
//     whose task details come from history_item.json, else the tasks'
//     _index.json; the session id is the directory name;
//   - storage/session_share/<session>.json, a cloud session marker;
//   - .config/kilo/{kilo.jsonc,kilo.json,opencode.jsonc,opencode.json,
//     config.json}: the permission key, read as JSON with comments;
//   - the sandbox policy and preference files under .local/state;
//   - retention/state.json, the last automatic deletion pass;
//   - memory/<folder>/sessions/<file>.md, project memory session digests,
//     with the folder's manifest.json canonical as the project path;
//   - log/<time>.log: the four remote control lines only.
//
// See research/kilo-code.md.
package kilocode

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/opencode"
)

// The artifacts the parser reads beyond its OpenCode stores, by path
// relative to the home: a task is searched for under any editor's user
// directory; the config files are those an "always" approval writes the
// permission key of; the sandbox files live beside the state directory.
var (
	taskPattern = regexp.MustCompile(
		`User/globalStorage/kilocode\.kilo-code/tasks/([^/]+)/api_conversation_history\.json\z`)
	configPattern = regexp.MustCompile(
		`^\.config/kilo/(?:kilo\.jsonc|kilo\.json|opencode\.jsonc|opencode\.json|config\.json)\z`)
	sandboxPolicyPattern     = regexp.MustCompile(`^\.local/state/kilo-sandbox-policy/[^/.][^/]*/[^/.][^/]*\.json\z`)
	sandboxPreferencePattern = regexp.MustCompile(`^\.local/state/kilo-sandbox-preference/[^/.][^/]*\.json\z`)
	sharePattern             = regexp.MustCompile(`^\.local/share/kilo/storage/session_share/([^/]+)\.json\z`)
	logPattern               = regexp.MustCompile(`^\.local/share/kilo/log/([^/]+\.log)\z`)
	retentionPattern         = regexp.MustCompile(`^\.local/share/kilo/retention/state\.json\z`)
	digestPattern            = regexp.MustCompile(`^\.local/share/kilo/memory/[^/]+/sessions/[^/]+\.md\z`)
)

// Parser reads Kilo Code's OpenCode stores, the pre-fork extension's tasks
// and Kilo's own settings, digests and logs.
type Parser struct {
	*opencode.Parser
}

// New returns the parser.
func New() *Parser {
	return &Parser{Parser: opencode.NewFor(opencode.Config{Agent: "kilo-code",
		DataDir: ".local/share/kilo", DatabasePrefixes: []string{"kilo", "opencode"},
		StateDir: ".local/state/kilo"})}
}

// IsTaskFile reports whether rel is a pre-fork task's
// api_conversation_history.json under any editor's user directory.
func IsTaskFile(rel string) bool {
	match := taskPattern.FindStringIndex(rel)
	return match != nil && (match[0] == 0 || rel[match[0]-1] == '/')
}

// Wants reports whether the artifact is one of Kilo's own files or one of
// its OpenCode stores.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	if IsTaskFile(rel) {
		return true
	}
	for _, pattern := range []*regexp.Regexp{sharePattern, logPattern, retentionPattern,
		digestPattern, configPattern, sandboxPolicyPattern, sandboxPreferencePattern} {
		if pattern.MatchString(rel) {
			return true
		}
	}
	return p.Parser.Wants(artifact)
}

// Parse emits the rows of one Kilo Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch match := sharePattern.FindStringSubmatch(rel); {
	case IsTaskFile(rel):
		p.task(artifact, options, emit)
	case match != nil:
		p.share(artifact, match[1], emit)
	case logPattern.MatchString(rel):
		p.log(artifact, emit)
	case retentionPattern.MatchString(rel):
		p.retention(artifact, emit)
	case digestPattern.MatchString(rel):
		p.digest(artifact, emit)
	case configPattern.MatchString(rel):
		p.config(artifact, emit)
	case sandboxPolicyPattern.MatchString(rel) || sandboxPreferencePattern.MatchString(rel):
		p.sandbox(artifact, emit)
	default:
		return p.Parser.Parse(artifact, options, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
