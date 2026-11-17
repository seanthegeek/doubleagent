package claudecode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// bridgeStatus is the line and URL of a system bridge_status record.
type bridgeStatus struct {
	line int
	url  string
}

// bridgeLine is the line of a bridge-session record and its bridge id,
// with named false when the record names none.
type bridgeLine struct {
	line     int
	bridgeID string
	named    bool
}

// prescan is what a transcript's first pass learns before its first row:
// the lines that deliver each queued prompt, whether the file is a fork,
// the first cwd and branch, the bridge_status and bridge-session records,
// whether an sdk-cli user record has promptSource sdk (a Remote Control
// spawn), and the line of the last remoteSourced record (a teleported
// cloud session; 0 when none), which ends the replayed block.
type prescan struct {
	delivered   map[string][]int
	fork        bool
	cwd, branch string
	bridgeURLs  []bridgeStatus
	bridgeLines []bridgeLine
	sdkSpawn    bool
	remoteLast  int
}

// prescanFile reads a transcript once for its prescan. Its problems are
// dropped: the second pass reports on the same file in full.
func prescanFile(artifact *input.Artifact) *prescan {
	scan := &prescan{delivered: map[string][]int{}}
	var ignored parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &ignored) {
		scan.record(line, record)
	}
	return scan
}

// record adds one record to the prescan.
func (s *prescan) record(line int, record jsonvalue.Value) {
	recordType := record.Get("type")
	if s.cwd == "" && !record.Get("cwd").IsEmpty() {
		s.cwd = record.Get("cwd").Text()
	}
	if s.branch == "" && !record.Get("gitBranch").IsEmpty() {
		s.branch = record.Get("gitBranch").Text()
	}
	if flag, ok := record.Get("remoteSourced").Bool(); ok && flag {
		s.remoteLast = line
	}
	if isText(recordType, "system") && isText(record.Get("subtype"), "bridge_status") &&
		!record.Get("url").IsEmpty() {
		s.bridgeURLs = append(s.bridgeURLs, bridgeStatus{line: line, url: record.Get("url").Text()})
	}
	if isText(recordType, "bridge-session") {
		bridgeID := record.Get("bridgeSessionId")
		named := bridgeID.IsString() && bridgeID.StringValue() != ""
		s.bridgeLines = append(s.bridgeLines, bridgeLine{line: line, bridgeID: bridgeID.StringValue(),
			named: named})
	}
	switch {
	case isText(recordType, "attachment"):
		attachment := record.Get("attachment")
		if isText(attachment.Get("type"), "queued_command") {
			s.deliver(parser.TextOf(attachment.Get("prompt")), line)
		}
	case isText(recordType, "user"):
		s.user(line, record)
	}
}

// user adds a user record's prompt texts to the prescan.
func (s *prescan) user(line int, record jsonvalue.Value) {
	if isText(record.Get("entrypoint"), "sdk-cli") && isText(record.Get("promptSource"), "sdk") {
		s.sdkSpawn = true
	}
	content := record.Get("message").Get("content")
	if content.IsString() {
		s.deliver(content.StringValue(), line)
		return
	}
	for _, block := range content.Items() {
		if !isText(block.Get("type"), "text") {
			continue
		}
		text := parser.Str(block.Get("text"))
		s.deliver(text, line)
		if strings.HasPrefix(pytext.StripLeft(text), forkMark) {
			s.fork = true
		}
	}
}

// deliver records that the line delivers a prompt with text.
func (s *prescan) deliver(text string, line int) {
	if text != "" {
		s.delivered[text] = append(s.delivered[text], line)
	}
}
