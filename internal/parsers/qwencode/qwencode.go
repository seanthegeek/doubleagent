package qwencode

import (
	"path"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/geminicli"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// workerHome is the managed local workers' $QWEN_HOME, a global directory
// of its own; the harness moves only its runtime base.
const workerHome = ".qwen/managed-local/worker-home/"

// runtimeRoots are the managed local mode's roots, each read as the .qwen/
// runtime base it mirrors.
func runtimeRoots() []string {
	return []string{".qwen/managed-local/harness/", workerHome}
}

// cutRoot returns rel below root, kept as the disk spells it, and true
// when rel starts with root ignoring the case of ASCII letters.
func cutRoot(rel, root string) (string, bool) {
	if len(rel) < len(root) || !textlines.EqualFoldASCII(rel[:len(root)], root) {
		return "", false
	}
	return rel[len(root):], true
}

// chatsExpression is a session's path without its suffix: a session id,
// group 1, in a project's chats/ or chats/archive/.
const chatsExpression = `^\.qwen/projects/[^/]+/chats/(?:archive/)?([0-9a-fA-F-]{32,36})`

// The artifacts the parser reads, by path relative to the home. The chat
// pattern is the session list's own file name test, which leaves out the
// sidecars.
var (
	sessionPattern      = parserkit.MustRelPattern(chatsExpression + `\.jsonl\z`)
	logsPattern         = parserkit.MustRelPattern(`^\.qwen/tmp/[^/]+/logs\.json\z`)
	tokenUsagePattern   = parserkit.MustRelPattern(`^\.qwen/usage/token-usage-[0-9]{4}-[0-9]{2}\.jsonl\z`)
	todoPattern         = parserkit.MustRelPattern(`^\.qwen/todos/[^/]+\.json\z`)
	organizationPattern = parserkit.MustRelPattern(`^\.qwen/projects/[^/]+/session-organization\.v1\.json\z`)
	pullRequestsPattern = parserkit.MustRelPattern(chatsExpression + `\.pr\.json\z`)
	trustedPattern      = parserkit.MustRelPattern(`^\.qwen/trustedFolders\.json\z`)
	channelsPattern     = parserkit.MustRelPattern(`^\.qwen/channels/sessions\.json\z`)
	daemonRoutesPattern = parserkit.MustRelPattern(`^\.qwen/channels/daemon/[^/]+/routes\.json\z`)
	groupHistoryPattern = parserkit.MustRelPattern(`^\.qwen/channels/[^/]+-group-history\.jsonl\z`)
	auditPattern        = parserkit.MustRelPattern(
		`^\.qwen/channels/[^/]+/[^/]+-[0-9a-f]{16}-github-audit\.jsonl\z`)
	settingsPattern = parserkit.MustRelPattern(`^\.qwen/settings\.json\z`)
	usagePattern    = parserkit.MustRelPattern(`^\.qwen/usage_record\.jsonl\z`)
	summaryPattern  = parserkit.MustRelPattern(`^\.qwen/PROJECT_SUMMARY\.md\z`)
)

// runtimePatterns are the files of the runtime base, matched on the path
// below a managed local root too.
func runtimePatterns() []*parserkit.RelPattern {
	return []*parserkit.RelPattern{sessionPattern, logsPattern, tokenUsagePattern, todoPattern,
		organizationPattern, pullRequestsPattern}
}

// homePatterns are the files matched on the path as it is.
func homePatterns() []*parserkit.RelPattern {
	return []*parserkit.RelPattern{trustedPattern, channelsPattern, daemonRoutesPattern,
		groupHistoryPattern, auditPattern, settingsPattern, usagePattern, summaryPattern}
}

// runtimeRel returns rel with a managed local root replaced by .qwen/, the
// runtime base it mirrors.
func runtimeRel(rel string) string {
	for _, root := range runtimeRoots() {
		if rest, ok := cutRoot(rel, root); ok {
			return ".qwen/" + rest
		}
	}
	return rel
}

// workerRel returns rel with the managed local workers' $QWEN_HOME
// replaced by .qwen/.
func workerRel(rel string) string {
	if rest, ok := cutRoot(rel, workerHome); ok {
		return ".qwen/" + rest
	}
	return rel
}

// Parser reads Qwen Code's chat files, prompt logs, settings, routes and
// usage records.
type Parser struct {
	parserkit.Base
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("qwen-code", "qwen-code")}
}

// Marks says the parser records sessions relayed through a messaging
// channel.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Relay: true}
}

// DedupHooks returns the match form and the match time under which a group
// history message meets the transcript's quote of it.
func (p *Parser) DedupHooks() parserkit.AgentHooks {
	return parserkit.AgentHooks{Form: MatchForm, Anchor: MatchAnchor}
}

// Wants reports whether the artifact is a chat file (not a ledger or
// runtime sidecar) or another file the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the runtime files, matched below a managed local root too,
// the files matched on the path as it is, and a managed worker's usage
// records.
func (p *Parser) Kinds() []parserkit.Kind {
	var kinds []parserkit.Kind
	runtimeNames := []string{"chat", "logs.json", "token usage", "todo", "session organization", "pull requests"}
	for i, pattern := range runtimePatterns() {
		kinds = append(kinds, parserkit.RelFuncKind(runtimeNames[i], func(rel string) bool {
			return pattern.MatchString(runtimeRel(rel))
		}))
	}
	homeNames := []string{"trustedFolders.json", "channels", "daemon routes", "group history", "audit",
		"settings.json", "usage_record.jsonl", "PROJECT_SUMMARY.md"}
	for i, pattern := range homePatterns() {
		kinds = append(kinds, parserkit.PatternKind(homeNames[i], pattern))
	}
	return append(kinds, parserkit.RelFuncKind("worker usage record", func(rel string) bool {
		return usagePattern.MatchString(workerRel(rel))
	}))
}

// Secondary reports whether the artifact is a prompt log, the usage
// records or a channel's group history.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return logsPattern.MatchString(runtimeRel(artifact.Rel)) ||
		usagePattern.MatchString(workerRel(artifact.Rel)) ||
		groupHistoryPattern.MatchString(artifact.Rel)
}

// Parse emits the rows of one Qwen Code file, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel, own := runtimeRel(artifact.Rel), artifact.Rel
	switch {
	case trustedPattern.MatchString(own):
		geminicli.TrustedFolderRows(p.Base, artifact, emit)
	case settingsPattern.MatchString(own):
		p.settings(artifact, emit)
	case usagePattern.MatchString(workerRel(own)):
		p.usage(artifact, emit)
	case channelsPattern.MatchString(own) || daemonRoutesPattern.MatchString(own):
		// Named without the leading .qwen/, as the row text names it.
		_, name, _ := strings.Cut(own, "/")
		p.channelRoutes(artifact, name, emit)
	case groupHistoryPattern.MatchString(own):
		p.groupHistory(artifact, emit)
	case auditPattern.MatchString(own):
		p.audit(artifact, emit)
	case summaryPattern.MatchString(own):
		p.summary(artifact, emit)
	case logsPattern.MatchString(rel):
		p.logs(artifact, emit)
	case tokenUsagePattern.MatchString(rel):
		p.tokenUsage(artifact, emit)
	case todoPattern.MatchString(rel):
		p.todos(artifact, emit)
	case organizationPattern.MatchString(rel):
		p.organization(artifact, emit)
	case pullRequestsPattern.MatchString(rel):
		p.pullRequests(artifact, emit)
	default:
		p.session(artifact, options, emit)
	}
	return nil
}

// fileName returns the artifact's file name, the last part of its path.
func fileName(artifact *input.Artifact) string {
	return path.Base(artifact.Rel)
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
	_ parserkit.HooksDeclarer   = (*Parser)(nil)
)
