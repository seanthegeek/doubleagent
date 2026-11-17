package geminicli

import (
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The tags a compression summary opens with: <state_snapshot>, or the
// reasoning the compression prompt asks for first, in a <scratchpad>.
const (
	scratchpadTag    = "<scratchpad>"
	stateSnapshotTag = "<state_snapshot>"
)

// hookContextEnd closes a hook's context, which a non-interactive start
// puts before the prompt in one text.
const hookContextEnd = "</hook_context>"

// contextOpening is the opening of a user text the CLI writes itself, and
// the kind of context row it gives.
type contextOpening struct {
	kind, opening string
}

// contextOpenings are the openings of the user texts the CLI writes itself
// that the parser recognises, matched against the trimmed text in order.
// The CLI's own reader skips the first two as no prompt. "Please
// continue." is not among them: a person can type it.
func contextOpenings() []contextOpening {
	return []contextOpening{
		{"session_context", "<session_context>"},
		{"hook_context", "<hook_context>"},
		{"state_snapshot", stateSnapshotTag},
		{"plan_mode_exit", "User has manually exited Plan Mode. Switching to"},
		{"directory_context", "- **Workspace Directories:**"},
		{"ide_context", "Here is the user's editor context as a JSON object."},
		{"ide_context", "Here is a summary of changes in the user's editor context, in JSON format."},
		{"shell_command", "I ran the following shell command:"},
		{"steering_update", "User steering update:"},
		{"user_hints", "User hints:"},
		{"loaded_context", "<loaded_context>"},
		{"background_update", "Background execution update:"},
	}
}

// parts returns a message's parts: an array's items, nothing for null, and
// any other value as the one part.
func parts(content jsonvalue.Value) []jsonvalue.Value {
	switch {
	case content.IsNull():
		return nil
	case content.IsArray():
		return content.Items()
	}
	return []jsonvalue.Value{content}
}

// partText returns the text of a Gemini message's parts, joined by LF with
// empty texts left out: a string part, a text part, a function response's
// text and a media part's label. With thoughts set it returns the text of
// the thought parts instead, and only theirs.
func partText(content jsonvalue.Value, thoughts bool) string {
	var out []string
	for _, part := range parts(content) {
		if part.IsString() {
			if !thoughts {
				out = append(out, part.StringValue())
			}
			continue
		}
		if !part.IsObject() || !part.Get("thought").IsEmpty() != thoughts {
			continue
		}
		switch text := part.Get("text"); {
		case text.IsString():
			out = append(out, text.StringValue())
		case thoughts:
		case part.Get("functionResponse").IsObject():
			out = append(out, responseText(part.Get("functionResponse")))
		default:
			out = append(out, mediaText(part))
		}
	}
	return joinNonEmpty(out)
}

// joinNonEmpty joins the non-empty texts with LF.
func joinNonEmpty(texts []string) string {
	kept := texts[:0:0]
	for _, text := range texts {
		if text != "" {
			kept = append(kept, text)
		}
	}
	return strings.Join(kept, "\n")
}

// contextKind returns the kind of a user text the CLI wrote itself, or ""
// for the person's own. The text is trimmed and its opening matched, so a
// tag typed inside a prompt does not count; a compression summary may
// instead open with <scratchpad> and hold <state_snapshot> after it.
func contextKind(text string) string {
	body := strings.TrimSpace(text)
	if strings.HasPrefix(body, scratchpadTag) && strings.Contains(body, stateSnapshotTag) {
		return "state_snapshot"
	}
	for _, each := range contextOpenings() {
		if strings.HasPrefix(body, each.opening) {
			return each.kind
		}
	}
	return ""
}

// run is a stretch of text of one context kind ("" for the person's).
type run struct {
	kind, text string
}

// classified returns a user text as runs: one, or each hook's context and
// the text after it. A text opening with a hook's context splits after its
// closing tag while text follows; each block is classified on its own
// slice, so the pass stays linear in the text's length.
func classified(text string) []run {
	var runs []run
	contentEnd := len(strings.TrimRightFunc(text, unicode.IsSpace))
	start := 0
	for {
		found := strings.Index(text[start:], hookContextEnd)
		tagEnd := start + found + len(hookContextEnd)
		if found < 0 || tagEnd >= contentEnd {
			break
		}
		block := text[start:tagEnd]
		if contextKind(block) != "hook_context" {
			break
		}
		runs = append(runs, run{"hook_context", block})
		start = tagEnd
	}
	rest := text[start:]
	return append(runs, run{contextKind(rest), rest})
}

// mediaText returns the label of an inlineData or a fileData part, or ""
// for any other part.
func mediaText(part jsonvalue.Value) string {
	if data := part.Get("inlineData"); data.IsObject() {
		return "[inlineData " + parser.Text(data.Get("mimeType")) + "]"
	}
	if data := part.Get("fileData"); data.IsObject() {
		return "[fileData " + parser.Text(data.Get("fileUri")) + "]"
	}
	return ""
}

// response is a functionResponse part with the labels of the media parts
// that follow it.
type response struct {
	value jsonvalue.Value
	media []string
}

// contentRuns returns the runs of a message's parts and its function
// responses with their media. A text takes its context kind when classify
// is set, else "". A media part before the first response is a "" text;
// after one it joins the nearest preceding response. Consecutive texts of
// one kind are one run, joined by LF; thought parts and empty texts give
// none.
func contentRuns(content jsonvalue.Value, classify bool) ([]run, []response) {
	var pieces []run
	var responses []response
	textRuns := func(text string) []run {
		if classify {
			return classified(text)
		}
		return []run{{"", text}}
	}
	for _, part := range parts(content) {
		switch {
		case part.IsString():
			pieces = append(pieces, textRuns(part.StringValue())...)
		case !part.IsObject() || !part.Get("thought").IsEmpty():
		case part.Get("text").IsString():
			pieces = append(pieces, textRuns(part.Get("text").StringValue())...)
		case part.Get("functionResponse").IsObject():
			responses = append(responses, response{value: part.Get("functionResponse")})
		default:
			if media := mediaText(part); media != "" && len(responses) > 0 {
				last := &responses[len(responses)-1]
				last.media = append(last.media, media)
			} else if media != "" {
				pieces = append(pieces, run{"", media})
			}
		}
	}
	return joinRuns(pieces), responses
}

// joinRuns merges consecutive pieces of one kind into one run, joined by
// LF, leaving out empty pieces.
func joinRuns(pieces []run) []run {
	var runs []run
	for _, piece := range pieces {
		if piece.text == "" {
			continue
		}
		if last := len(runs) - 1; last >= 0 && runs[last].kind == piece.kind {
			runs[last].text += "\n" + piece.text
			continue
		}
		runs = append(runs, piece)
	}
	return runs
}

// responseText returns the text of a functionResponse: its response's
// non-empty output, error or content string, a string response as it is,
// else the response as compact JSON.
func responseText(functionResponse jsonvalue.Value) string {
	value := functionResponse.Get("response")
	if value.IsObject() {
		for _, key := range []string{"output", "error", "content"} {
			if text := value.Get(key); text.IsString() && text.StringValue() != "" {
				return text.StringValue()
			}
		}
	}
	if value.IsString() {
		return value.StringValue()
	}
	return parser.JSONText(value)
}

// resultText returns a tool_result text: the response's text, then each of
// its media labels, one per line.
func (r response) resultText() string {
	return joinNonEmpty(append([]string{responseText(r.value)}, r.media...))
}

// callResultText returns the text of a tool call's result. A result holding
// a functionResponse whose id is the call's gives that response's text and
// media, nothing else, since a history sync rewrites a result to every part
// of the answering turn. Any other result gives every part's text, joined,
// a media part as its label and anything else as compact JSON.
func callResultText(result jsonvalue.Value, callID string) string {
	switch {
	case result.IsNull():
		return ""
	case result.IsString():
		return result.StringValue()
	}
	_, responses := contentRuns(result, false)
	for _, each := range responses {
		if parser.Text(each.value.Get("id")) == callID {
			return each.resultText()
		}
	}
	var texts []string
	for _, part := range parts(result) {
		switch {
		case part.IsObject() && part.Get("functionResponse").IsObject():
			texts = append(texts, responseText(part.Get("functionResponse")))
		case part.IsObject() && part.Get("text").IsString():
			texts = append(texts, part.Get("text").StringValue())
		case part.IsString():
			texts = append(texts, part.StringValue())
		default:
			media := ""
			if part.IsObject() {
				media = mediaText(part)
			}
			if media == "" {
				media = parser.JSONText(part)
			}
			texts = append(texts, media)
		}
	}
	return joinNonEmpty(texts)
}

// toolText returns the tool_use text of a tool call: its arguments as
// compact JSON, after the display name when it is set and differs from the
// call's name.
func toolText(call jsonvalue.Value) string {
	arguments := call.Get("args")
	argumentsText := ""
	if !arguments.IsNull() && !(arguments.IsString() && arguments.StringValue() == "") {
		argumentsText = parser.JSONText(arguments)
	}
	label := call.Get("displayName")
	if !label.IsEmpty() && !equal(label, call.Get("name")) {
		if argumentsText != "" {
			return label.Text() + ": " + argumentsText
		}
		return label.Text()
	}
	return argumentsText
}
