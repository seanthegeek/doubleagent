package geminicli

import (
	"regexp"

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
	sessionPattern    = regexp.MustCompile(tempExpression + `chats/(?:.*/)?[^/]+\.jsonl?\z`)
	logsPattern       = regexp.MustCompile(tempExpression + `logs\.json\z`)
	trustedPattern    = regexp.MustCompile(baseExpression + `trustedFolders\.json\z`)
	shellPattern      = regexp.MustCompile(tempExpression + `shell_history\z`)
	checkpointPattern = regexp.MustCompile(tempExpression + `checkpoint-(?P<tag>[^/]+)\.json\z`)
	bugPattern        = regexp.MustCompile(tempExpression +
		`bug-report-history-(?P<ms>[0-9]+)\.json\z`)
	activityPattern = regexp.MustCompile(tempExpression +
		`logs/session-(?P<session>[^/]+)\.jsonl\z`)
	toolCheckpointPattern = regexp.MustCompile(tempExpression + `checkpoints/(?P<time>[0-9]{4}-` +
		`[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}_[0-9]{3}Z)-[^/]+\.json\z`)
	memoryPattern    = regexp.MustCompile(tempExpression + `memory/(?P<name>.+)\z`)
	clipboardPattern = regexp.MustCompile(tempExpression +
		`images/clipboard-(?P<ms>[0-9]+)\.(?:png|jpg|jpeg|webp|heic|heif)\z`)
	errorReportPattern        = regexp.MustCompile(baseExpression + errorReportName)
	projectErrorReportPattern = regexp.MustCompile(tempExpression + errorReportName)
	// The standing files: settings.json and the policies in the home's
	// .gemini and in a project's, the other two in the home's only.
	settingsPattern  = regexp.MustCompile(`^\.gemini/settings\.json\z`)
	policyPattern    = regexp.MustCompile(`^\.gemini/policies/[^/]+\.toml(?:\.bak)?\z`)
	hooksPattern     = regexp.MustCompile(`^\.gemini/trusted_hooks\.json\z`)
	agentsAckPattern = regexp.MustCompile(`^\.gemini/acknowledgments/agents\.json\z`)
)

// pathMatch holds the named groups of a pattern that matched a path.
type pathMatch map[string]string

// matchPath returns the named groups of pattern in rel, or nil when it does
// not match.
func matchPath(pattern *regexp.Regexp, rel string) pathMatch {
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

// Wants reports whether the artifact is one of the files the parser reads;
// trusted_hooks.json and acknowledgments/agents.json only in a user's home.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
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
		pattern *regexp.Regexp
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

// Secondary reports whether the artifact repeats events of the session
// files: a prompt log, the shell history, a history copy, a tool checkpoint
// or an error report.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	for _, pattern := range []*regexp.Regexp{logsPattern, shellPattern, checkpointPattern,
		bugPattern, toolCheckpointPattern, errorReportPattern, projectErrorReportPattern} {
		if pattern.MatchString(artifact.Rel) {
			return true
		}
	}
	return false
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

// firstMatch returns the groups of the first pattern that matches rel, or
// nil.
func firstMatch(rel string, patterns ...*regexp.Regexp) pathMatch {
	for _, pattern := range patterns {
		if match := matchPath(pattern, rel); match != nil {
			return match
		}
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
