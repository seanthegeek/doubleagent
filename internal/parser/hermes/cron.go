package hermes

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// The headings of a cron run document's prompt and response sections.
const (
	promptHeading   = "## Prompt\n\n"
	responseHeading = "## Response\n\n"
)

// splitCronDocument returns the prompt and the response of a cron run
// document, or false when it has neither (a failed, blocked or
// script-gated run). The length frames ("**Prompt Characters:** <n>",
// "**Response Characters:** <n>", lengths in characters) are used when the
// writer stamped them, else the first prompt heading and the last response
// heading.
func (p *Parser) splitCronDocument(document string) (string, string, bool) {
	if frame := p.promptFrame.FindStringSubmatchIndex(document); frame != nil &&
		strings.Index(document, promptHeading) == frame[1]-len(promptHeading) {
		end := advance(document, frame[1], document[frame[2]:frame[3]])
		offset := advance(document, end, "2")
		if reply := p.responseFrame.FindStringSubmatchIndex(document[offset:]); reply != nil {
			replyEnd := offset + reply[1]
			length := document[offset+reply[2] : offset+reply[3]]
			return document[frame[1]:end], document[replyEnd:advance(document, replyEnd, length)], true
		}
	}
	start := strings.Index(document, promptHeading)
	cut := strings.LastIndex(document, responseHeading)
	if start < 0 || cut <= start {
		return "", "", false
	}
	prompt := document[min(start+len(promptHeading), cut):cut]
	prompt = strings.TrimSuffix(prompt, "\n\n")
	reply := strings.TrimSuffix(document[cut+len(responseHeading):], "\n")
	return prompt, reply, true
}

// advance returns the byte offset count characters after the byte offset
// start of text, count given in decimal digits, or the end of text when it
// has fewer characters left. A byte that is not UTF-8 counts as one
// character.
func advance(text string, start int, count string) int {
	n, err := strconv.Atoi(count)
	if err != nil || n >= len(text)-start {
		return len(text)
	}
	offset := start
	for range n {
		_, size := utf8.DecodeRuneInString(text[offset:])
		offset += size
	}
	return offset
}
