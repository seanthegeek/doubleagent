// Package geminicli reads Gemini CLI's sessions, the past-use stores beside
// them and its standing permission files, under ~/.gemini or, under the
// macOS sandbox, ~/.cache/.gemini.
//
// The sessions are tmp/<slug>/chats/**/*.jsonl, a log of messages and of
// the mutations ($set, $patch, $rewindTo) the CLI's recording service
// writes, replayed here as the CLI replays them, and the legacy whole-JSON
// .json form. The secondary sources repeat some of their events:
// logs.json, shell_history, the /chat save checkpoint-<tag>.json, the /bug
// bug-report-history-<ms>.json, the tool checkpoints checkpoints/*.json and
// a copied gemini-client-error-<type>-<time>.json. The activity log
// logs/session-<id>.jsonl (its records, not the turns in their bodies), the
// memory/ files and the clipboard images images/clipboard-<ms>.<ext> are
// primary. The standing grants are trustedFolders.json, settings.json,
// policies/*.toml with sandbox.toml, trusted_hooks.json and
// acknowledgments/agents.json.
//
// A row's project is tmp/<slug>/.project_root, else the projects.json key
// of the slug, else the key whose sha256 is the hash directory or the
// session's projectHash, else the projectHash itself. See
// research/gemini-cli.md.
//
// TrustedFolderRows reads a trustedFolders.json for Qwen Code's parser too,
// whose fork keeps the file's form.
package geminicli

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	sessionPattern        = regexp.MustCompile(tempExpression + `chats/(?:.*/)?[^/]+\.jsonl?\z`)
	logsPattern           = regexp.MustCompile(tempExpression + `logs\.json\z`)
	trustedPattern        = regexp.MustCompile(baseExpression + `trustedFolders\.json\z`)
	shellPattern          = regexp.MustCompile(tempExpression + `shell_history\z`)
	checkpointPattern     = regexp.MustCompile(tempExpression + `checkpoint-(?P<tag>[^/]+)\.json\z`)
	bugPattern            = regexp.MustCompile(tempExpression + `bug-report-history-(?P<ms>[0-9]+)\.json\z`)
	activityPattern       = regexp.MustCompile(tempExpression + `logs/session-(?P<session>[^/]+)\.jsonl\z`)
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
	parser.Base
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parser.NewBase("gemini-cli", "gemini-cli")}
}

// Wants reports whether the artifact is one of the files the parser reads;
// trusted_hooks.json and acknowledgments/agents.json only in a user's home.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	if hooksPattern.MatchString(rel) || agentsAckPattern.MatchString(rel) {
		return !artifact.Home.Project
	}
	for _, pattern := range []*regexp.Regexp{sessionPattern, logsPattern, trustedPattern,
		shellPattern, checkpointPattern, bugPattern, settingsPattern, policyPattern,
		activityPattern, toolCheckpointPattern, memoryPattern, clipboardPattern,
		errorReportPattern, projectErrorReportPattern} {
		if pattern.MatchString(rel) {
			return true
		}
	}
	return false
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
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
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
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
