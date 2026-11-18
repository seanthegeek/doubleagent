package continuedev

import (
	"path"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// nonSpace is one character that is not whitespace by unicode.IsSpace.
const nonSpace = `[^` + parser.Space + `]`

// The lines of a cn log that are read. A line is
// "<date> <time.ms> [<process id>] [<level>]: <message>". A [metadata]
// result line (extensions/cli/src/util/metadata.ts:144-180) has the status
// word, then the URL, which ends /metadata, followed on a Failed line by
// the response as JSON. The status word must be a whole word: what follows
// it, if anything stands before the space and the URL, starts with a
// character that is not a letter, digit or underscore of any script (Go's
// \b knows ASCII letters only).
var (
	logLine      = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}) \[([0-9a-fA-F]{8})\] \[[A-Za-z]+\]: (.*)\z`)
	metadataPost = regexp.MustCompile(`^\[metadata\] POST (` + nonSpace + `+)(?: (\{.*\}))?\z`)
	metadataEnd  = regexp.MustCompile(`^\[metadata\] (Success|Unexpected response|Auth required|Failed|Error)` +
		`(?:[^\p{L}\p{N}_].*)? (` + nonSpace + `+/metadata)(?: \{.*\})?\z`)
	agentURL    = regexp.MustCompile(`/agents/([^/]+)/metadata\z`)
	policyAdded = regexp.MustCompile(`^Added policy rule to permissions\.yaml: (.*)\z`)
)

// storageSkipped is in the line the CLI logs when it skips a storage sync.
const storageSkipped = "Storage sync requested but no Continue API key"

// post is one metadata POST of a cn log: its process and URL, the fields
// it sent, the status its result line gave, and the index of its row.
type post struct {
	processID, url, fields string
	status                 string
	row                    int
}

// cnLog emits the "agent metadata posted:", "storage upload requested" and
// "permission decision: allow always" rows of a cn log. A post's row waits
// for the result line of its process and URL to learn its status.
func (p *Parser) cnLog(artifact *input.Artifact, emit parser.Emit) {
	source := "logs/" + path.Base(artifact.Rel)
	var problems parser.Problems
	var rows []timeline.Row
	var posts []*post
	pending := newPendingPosts(p.Work)
	for line, raw := range parser.Lines(artifact.DiskPath, &problems) {
		match := logLine.FindStringSubmatch(parser.LineText(raw))
		if match == nil {
			continue
		}
		when, processID, message := match[1], match[2], match[3]
		row := p.SystemRow(artifact, "")
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromText(when)
		if strings.Contains(message, storageSkipped) {
			row.Text = "storage upload requested, skipped (from " + source + ", log line time, host-local)"
		} else if added := policyAdded.FindStringSubmatch(message); added != nil {
			row.Text = "permission decision: allow always rule=" + added[1] + " (from " + source +
				", log line time, host-local)"
		} else if sent := metadataPost.FindStringSubmatchIndex(message); sent != nil {
			entry := &post{processID: processID, url: message[sent[2]:sent[3]], status: "-", row: len(rows)}
			if sent[4] >= 0 {
				entry.fields = message[sent[4]:sent[5]]
			}
			posts = append(posts, entry)
			pending.add(entry)
		} else {
			if result := metadataEnd.FindStringSubmatch(message); result != nil {
				pending.settle(processID, result[2], result[1])
			}
			continue
		}
		rows = append(rows, row)
	}
	for _, entry := range posts {
		rows[entry.row].Text = postText(entry, source)
	}
	parser.EmitAll(emit, rows)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// postKey is the process and URL a result line answers.
type postKey struct {
	processID, url string
}

// pendingPosts holds the posts that wait for their result line, oldest
// first per process and URL, so a result finds its post without a scan of
// the others. work counts the posts a result was given to, at most one per
// result line, which a test reads.
type pendingPosts struct {
	queues map[postKey][]*post
	work   parser.Work
}

// newPendingPosts returns an empty set of pending posts that counts its
// work in work.
func newPendingPosts(work parser.Work) *pendingPosts {
	return &pendingPosts{queues: map[postKey][]*post{}, work: work}
}

// add puts a post at the end of its process and URL's queue.
func (q *pendingPosts) add(entry *post) {
	key := postKey{entry.processID, entry.url}
	q.queues[key] = append(q.queues[key], entry)
}

// settle gives the oldest pending post of a process to a URL its status
// and takes it off the queue; a result no post waits for changes nothing.
func (q *pendingPosts) settle(processID, url, status string) {
	key := postKey{processID, url}
	queue := q.queues[key]
	if len(queue) == 0 {
		return
	}
	q.work.Add("posts settled", 1)
	queue[0].status = status
	if len(queue) == 1 {
		delete(q.queues, key)
		return
	}
	q.queues[key] = queue[1:]
}

// postText returns the row text of a metadata post: the agent, from the
// fields sent or else the URL, the metadata keys sent and the status.
func postText(entry *post, source string) string {
	document := jsonvalue.NewNull()
	if entry.fields != "" {
		if decoded, err := jsonvalue.DecodeString(entry.fields); err == nil {
			document = decoded
		}
	}
	agent := "-"
	if value := document.Get("agentId"); value.IsString() && value.StringValue() != "" {
		agent = value.StringValue()
	} else if match := agentURL.FindStringSubmatch(entry.url); match != nil {
		agent = match[1]
	}
	keys := "-"
	if list := document.Get("metadataKeys"); list.IsArray() && list.Len() > 0 {
		names := make([]string, list.Len())
		for i, key := range list.Items() {
			names[i] = key.Text()
		}
		keys = strings.Join(names, ",")
	}
	return "agent metadata posted: agent " + agent + " keys " + keys + " " + entry.status +
		" (from " + source + ", log line time, host-local)"
}
