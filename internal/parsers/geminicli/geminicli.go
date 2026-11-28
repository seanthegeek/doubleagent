package geminicli

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The runtime directory, .gemini or .cache/.gemini, and the per-project
// directory tmp/<slug> below it that holds the sessions and their past-use
// copies.
const (
	baseExpression = `^(?P<base>(?:\.cache/)?\.gemini)/`
	tempExpression = baseExpression + `tmp/(?P<slug>[^/]+)/`
	// An error report's name ends with an ISO time, ":" and "." both
	// written "-"; the CLI writes it to the system's temporary directory and
	// the analyst places a copy in the runtime or a project's directory.
	errorReportName = `gemini-client-error-(?P<type>[^/]+)-` +
		`(?P<time>[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{3}Z)\.json\z`
)

// The artifacts the parser reads, by path relative to the home. A tool
// checkpoint's name opens with an ISO time, ":" written "-" and "." "_".
var (
	sessionPattern    = parserkit.MustRelPattern(tempExpression + `chats/(?:.*/)?[^/]+\.jsonl?\z`)
	logsPattern       = parserkit.MustRelPattern(tempExpression + `logs\.json\z`)
	trustedPattern    = parserkit.MustRelPattern(baseExpression + `trustedFolders\.json\z`)
	shellPattern      = parserkit.MustRelPattern(tempExpression + `shell_history\z`)
	checkpointPattern = parserkit.MustRelPattern(tempExpression + `checkpoint-(?P<tag>[^/]+)\.json\z`)
	bugPattern        = parserkit.MustRelPattern(tempExpression +
		`bug-report-history-(?P<ms>[0-9]+)\.json\z`)
	activityPattern = parserkit.MustRelPattern(tempExpression +
		`logs/session-(?P<session>[^/]+)\.jsonl\z`)
	toolCheckpointPattern = parserkit.MustRelPattern(tempExpression + `checkpoints/(?P<time>[0-9]{4}-` +
		`[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}_[0-9]{3}Z)-[^/]+\.json\z`)
	memoryPattern    = parserkit.MustRelPattern(tempExpression + `memory/(?P<name>.+)\z`)
	clipboardPattern = parserkit.MustRelPattern(tempExpression +
		`images/clipboard-(?P<ms>[0-9]+)\.(?:png|jpg|jpeg|webp|heic|heif)\z`)
	errorReportPattern        = parserkit.MustRelPattern(baseExpression + errorReportName)
	projectErrorReportPattern = parserkit.MustRelPattern(tempExpression + errorReportName)
	// The standing files: settings.json and the policies in the home's
	// .gemini and in a project's, the other two in the home's only.
	settingsPattern  = parserkit.MustRelPattern(`^\.gemini/settings\.json\z`)
	policyPattern    = parserkit.MustRelPattern(`^\.gemini/policies/[^/]+\.toml(?:\.bak)?\z`)
	hooksPattern     = parserkit.MustRelPattern(`^\.gemini/trusted_hooks\.json\z`)
	agentsAckPattern = parserkit.MustRelPattern(`^\.gemini/acknowledgments/agents\.json\z`)
)

// pathMatch holds the named groups of a pattern that matched a path.
type pathMatch map[string]string

// matchPath returns the named groups of pattern in rel, or nil when it does
// not match.
func matchPath(pattern *parserkit.RelPattern, rel string) pathMatch {
	found := pattern.FindStringSubmatch(rel)
	if found == nil {
		return nil
	}
	groups := pathMatch{}
	for i, name := range pattern.SubexpNames() {
		if name != "" {
			groups[name] = found[i]
		}
	}
	return groups
}

// Parser reads Gemini CLI's sessions, prompt logs, history copies and
// standing permission files.
type Parser struct {
	parserkit.Base
	projects map[fileState]*projectIndex
	// roots holds each .project_root read, by its state.
	roots map[fileState]*sideText
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("gemini-cli", "gemini-cli"), projects: map[fileState]*projectIndex{}}
}

// Kinds returns the files the parser reads. The trusted hooks and the
// agents acknowledgments are read in a home only, and a path that names
// either is no other kind.
func (p *Parser) Kinds() []parserkit.Kind {
	acknowledged := func(rel string) bool { return hooksPattern.MatchString(rel) || agentsAckPattern.MatchString(rel) }
	kinds := []parserkit.Kind{
		{Name: "trusted_hooks.json", Wants: func(artifact *input.Artifact) bool {
			return hooksPattern.MatchString(artifact.Rel) && !artifact.Home.Project
		}},
		{Name: "agents acknowledgments", Wants: func(artifact *input.Artifact) bool {
			return agentsAckPattern.MatchString(artifact.Rel) && !artifact.Home.Project
		}}}
	for _, each := range []struct {
		name    string
		pattern *parserkit.RelPattern
	}{{"session", sessionPattern}, {"logs.json", logsPattern}, {"trustedFolders.json", trustedPattern},
		{"shell_history", shellPattern}, {"checkpoint", checkpointPattern}, {"bug report history", bugPattern},
		{"settings.json", settingsPattern}, {"policy", policyPattern}, {"session activity log", activityPattern},
		{"tool checkpoint", toolCheckpointPattern}, {"memory", memoryPattern}, {"clipboard image", clipboardPattern},
		{"error report", errorReportPattern}, {"project error report", projectErrorReportPattern}} {
		kinds = append(kinds, parserkit.RelFuncKind(each.name, func(rel string) bool {
			return !acknowledged(rel) && each.pattern.MatchString(rel)
		}))
	}
	return kinds
}

// Wants reports whether the artifact is one of the files the parser reads;
// trusted_hooks.json and acknowledgments/agents.json only in a user's home.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact repeats events of the session
// files: a prompt log, the shell history, a history copy, a tool checkpoint
// or an error report.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	for _, pattern := range []*parserkit.RelPattern{logsPattern, shellPattern, checkpointPattern,
		bugPattern, toolCheckpointPattern, errorReportPattern, projectErrorReportPattern} {
		if pattern.MatchString(artifact.Rel) {
			return true
		}
	}
	return false
}

// firstMatch returns the groups of the first pattern that matches rel, or
// nil.
func firstMatch(rel string, patterns ...*parserkit.RelPattern) pathMatch {
	for _, pattern := range patterns {
		if match := matchPath(pattern, rel); match != nil {
			return match
		}
	}
	return nil
}

// Parse emits the rows of one Gemini CLI file, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	if match := firstMatch(rel, errorReportPattern, projectErrorReportPattern); match != nil {
		p.errorReport(artifact, options, match, emit)
	} else if match := matchPath(activityPattern, rel); match != nil {
		p.activityLog(artifact, match, emit)
	} else if match := matchPath(toolCheckpointPattern, rel); match != nil {
		p.toolCheckpoint(artifact, options, match, emit)
	} else if match := matchPath(memoryPattern, rel); match != nil {
		p.memoryFile(artifact, match, emit)
	} else if match := matchPath(clipboardPattern, rel); match != nil {
		p.clipboardImage(artifact, match, emit)
	} else if trustedPattern.MatchString(rel) {
		TrustedFolderRows(p.Base, artifact, emit)
	} else if settingsPattern.MatchString(rel) {
		p.settings(artifact, emit)
	} else if policyPattern.MatchString(rel) {
		p.policy(artifact, emit)
	} else if hooksPattern.MatchString(rel) || agentsAckPattern.MatchString(rel) {
		p.projectGrants(artifact, emit)
	} else if match := matchPath(logsPattern, rel); match != nil {
		p.logs(artifact, match, emit)
	} else if match := matchPath(shellPattern, rel); match != nil {
		p.shellHistory(artifact, match, emit)
	} else if match := firstMatch(rel, checkpointPattern, bugPattern); match != nil {
		p.historyCopy(artifact, options, match, emit)
	} else if match := matchPath(sessionPattern, rel); match != nil {
		p.session(artifact, options, match, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
