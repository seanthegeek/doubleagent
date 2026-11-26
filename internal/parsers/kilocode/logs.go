package kilocode

import (
	"cmp"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// nonSpace is one character that is not whitespace by unicode.IsSpace: a
// field value ends at any Unicode space, the no-break space included.
const nonSpace = `[^` + parserkit.Space + `]`

// The remote control messages a log line ends with, in the order they are
// looked for.
const (
	connectionEnabled  = "remote connection enabled"
	connectionDisabled = "remote connection disabled"
	sessionStarted     = "remote session started"
	sessionEnded       = "remote session ended"
)

// remoteMessages returns the remote control messages in the order a line's
// end is tested against them.
func remoteMessages() []string {
	return []string{connectionEnabled, connectionDisabled, sessionStarted, sessionEnded}
}

// A log line is "<LEVEL> <UTC time to the second> +<n>ms <key>=<value>...
// <message>"; each remote control message has its own fields.
var (
	logHeadPattern = regexp.MustCompile(
		`^(?:DEBUG|INFO|WARN|ERROR)[ \t]+([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}) \+[0-9]+ms `)
	enabledPattern  = regexp.MustCompile(` connected=(` + nonSpace + `+) remote connection enabled\z`)
	disabledPattern = regexp.MustCompile(` remote connection disabled\z`)
	startedPattern  = regexp.MustCompile(` sessionID=(` + nonSpace + `+) startedAt=(` + nonSpace + `+)` +
		`(?: model=(` + nonSpace + `+))?(?: directory=(.+?))? remote session started\z`)
	endedPattern = regexp.MustCompile(` sessionID=(` + nonSpace + `+) startedAt=(` + nonSpace + `+)` +
		` durationMs=([0-9]+) reason=(` + nonSpace + `+) remote session ended\z`)
)

// log emits one "remote control:" row per remote control line of
// log/<time>.log: connection enabled and disabled at the line's time,
// session started at its startedAt with the session and directory, session
// ended at the line's time with the session. Remote control lets Kilo Cloud
// drive sessions on this host while the transcript stays local, so these
// are not sync rows. A line ending with one of the messages but lacking its
// fields is a `parser: unparseable line` row; no other line is read.
func (p *Parser) log(artifact *input.Artifact, emit parserkit.Emit) {
	name := artifact.Rel[strings.LastIndex(artifact.Rel, "/")+1:]
	var problems parserkit.Problems
	data, _ := parserkit.FileBytes(artifact.DiskPath, &problems)
	for number, raw := range parserkit.SplitLines(data) {
		line := parserkit.LineText(raw)
		message := ""
		for _, candidate := range remoteMessages() {
			if strings.HasSuffix(line, " "+candidate) {
				message = candidate
				break
			}
		}
		if message == "" {
			continue
		}
		row, ok := p.remoteRow(artifact, name, number, line, message)
		if !ok {
			problems.AddRaw(number, "unparseable line ("+message+" without its fields)", line)
			continue
		}
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// remoteRow returns the row of one remote control line, or false when the
// line lacks its fields.
func (p *Parser) remoteRow(artifact *input.Artifact, name string, number int, line,
	message string) (timeline.Row, bool) {
	head := logHeadPattern.FindStringSubmatch(line)
	if head == nil {
		return timeline.Row{}, false
	}
	pattern := map[string]*regexp.Regexp{connectionEnabled: enabledPattern,
		connectionDisabled: disabledPattern, sessionStarted: startedPattern,
		sessionEnded: endedPattern}[message]
	fields := pattern.FindStringSubmatch(line)
	if fields == nil {
		return timeline.Row{}, false
	}
	row := p.SystemRow(artifact, "")
	row.SourceLine = number
	row.TimestampUTC = timeutil.FromText(head[1])
	text, basis := "", "line time"
	switch message {
	case connectionEnabled:
		text = "connection enabled connected=" + fields[1]
	case connectionDisabled:
		text = "connection disabled"
	case sessionStarted:
		row.SessionID, row.ProjectPath = fields[1], fields[4]
		row.TimestampUTC = cmp.Or(timeutil.FromText(fields[2]), row.TimestampUTC)
		text = "session started model=" + cmp.Or(fields[3], "-") + " directory=" + cmp.Or(fields[4], "-")
		basis = "startedAt"
	default:
		row.SessionID = fields[1]
		text = "session ended reason=" + fields[4] + " durationMs=" + fields[3] + " startedAt=" + fields[2]
	}
	row.Text = timeline.Compact("remote control: " + text + " (from log/" + name + ", " + basis + ")")
	return row, true
}

// digest emits the "memory digest:" row of one project memory session
// digest at its "Updated:" line: the session id from its "# Session <id>"
// heading, the summary every line after "## Summary", and the project path
// the canonical key of the folder's manifest.json. A digest that cannot be
// read or has no "## Summary" line, or a manifest that is not a JSON
// object, is a `parser:` row.
func (p *Parser) digest(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	project := ""
	if manifest, found := parserkit.Above(artifact, 1, "manifest.json"); found {
		project = p.manifestRoot(artifact, manifest, &problems)
	}
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	var lines []string
	for _, raw := range parserkit.SplitLines(data) {
		lines = append(lines, parserkit.LineText(raw))
	}
	fields, summaryAt := digestFields(lines)
	sessionID := fields["# Session "]
	if summaryAt < 0 {
		problems.Add(0, artifact.Rel+" is not a memory digest (no ## Summary line)")
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
		return
	}
	row := p.SystemRow(artifact, "")
	row.SessionID, row.ProjectPath = sessionID, project
	row.SourceLine = summaryAt + 2
	row.TimestampUTC = timeutil.FromText(fields["Updated: "])
	summary := strings.Trim(strings.Join(lines[summaryAt+1:], "\n"), " \t\n")
	row.Text = "memory digest: " + cmp.Or(fields["Topic: "], "-") + ": " + summary + " (from " +
		artifact.Rel + ", Updated)"
	emit(row)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}

// digestFields returns the first non-empty value of each heading field of
// a digest's lines before its "## Summary" line, and that line's index, -1
// when there is none.
func digestFields(lines []string) (map[string]string, int) {
	fields := map[string]string{"# Session ": "", "Updated: ": "", "Topic: ": ""}
	for number, line := range lines {
		if strings.Trim(line, " \t") == "## Summary" {
			return fields, number
		}
		for _, prefix := range []string{"# Session ", "Updated: ", "Topic: "} {
			if strings.HasPrefix(line, prefix) && fields[prefix] == "" {
				fields[prefix] = strings.Trim(line[len(prefix):], " \t")
			}
		}
	}
	return fields, -1
}

// manifestFile is a manifest.json read once: its canonical root and the
// problems reading it added.
type manifestFile struct {
	root     string
	problems parserkit.Problems
}

// manifestRoot returns readManifest's root of the manifest.json at path,
// read once per state of the file for the parser's life, since every
// digest of the memory folder looks it up; the problems of that read are
// added for each lookup as a fresh read would.
func (p *Parser) manifestRoot(artifact *input.Artifact, path string, problems *parserkit.Problems) string {
	info, err := osfile.Lstat(path)
	if err != nil {
		return ""
	}
	key := indexKey{path: path, mtime: info.ModTime().UnixNano(), size: info.Size()}
	read, ok := p.manifests[key]
	if !ok {
		p.Work.Add("manifest.json reads", 1)
		read = &manifestFile{}
		read.root = readManifest(artifact, path, &read.problems)
		if p.manifests == nil {
			p.manifests = map[indexKey]*manifestFile{}
		}
		p.manifests[key] = read
	}
	*problems = append(*problems, read.problems...)
	return read.root
}

// readManifest returns the canonical project root of a memory folder's
// manifest.json, or "". An absent manifest, or one without canonical, is
// silent; one that cannot be read or is not a JSON object adds a problem.
func readManifest(artifact *input.Artifact, path string, problems *parserkit.Problems) string {
	if !parserkit.RegularFile(path) {
		return ""
	}
	document, ok := readJSON(artifact, path, problems)
	if !ok {
		return ""
	}
	if !document.IsObject() {
		problems.Add(0, "memory manifest.json is not a JSON object")
		return ""
	}
	if canonical := document.Get("canonical"); canonical.IsString() {
		return canonical.StringValue()
	}
	return ""
}
