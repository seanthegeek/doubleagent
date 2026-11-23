package kilocode

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/opencode"
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
	indexes map[indexKey]*taskIndex
	// manifests holds each memory folder's manifest.json read, by its
	// state.
	manifests map[indexKey]*manifestFile
}

// New returns the parser.
func New() *Parser {
	return &Parser{Parser: opencode.NewFor(opencode.Config{Agent: "kilo-code",
		DataDir: ".local/share/kilo", DatabasePrefixes: []string{"kilo", "opencode"},
		StateDir: ".local/state/kilo"}), indexes: map[indexKey]*taskIndex{}}
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
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the pre-fork task files, Kilo's own files and the kinds of
// its OpenCode stores.
func (p *Parser) Kinds() []parserkit.Kind {
	return append([]parserkit.Kind{parserkit.RelFuncKind("pre-fork task", IsTaskFile),
		parserkit.PatternKind("session share", sharePattern), parserkit.PatternKind("log", logPattern),
		parserkit.PatternKind("retention state.json", retentionPattern), parserkit.PatternKind("memory digest", digestPattern),
		parserkit.PatternKind("config", configPattern), parserkit.PatternKind("sandbox policy", sandboxPolicyPattern),
		parserkit.PatternKind("sandbox preference", sandboxPreferencePattern)}, p.Parser.Kinds()...)
}

// Parse emits the rows of one Kilo Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
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
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
