package parsers

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The Aider growth cases: the chat history with the input history beside
// it, whose entries give its prompts their times, the input history and
// the LLM history, both of which read the chat history's session headers.

// The three files, in one project directory.
const (
	aiderChat  = "p/.aider.chat.history.md"
	aiderInput = "p/.aider.input.history"
	aiderLLM   = "p/.aider.llm.history"
)

// aiderStamp returns the local time of second i of a day, as Aider writes it
// after "# " or "aider chat started at ".
func aiderStamp(i int) string {
	return fmt.Sprintf("2026-01-02 %02d:%02d:%02d", i/3600%24, i/60%60, i%60)
}

// aiderHeader returns a chat history's session header at second i.
func aiderHeader(i int) string {
	return "# aider chat started at " + aiderStamp(i) + "\n"
}

// aiderInputEntry returns an input history entry at second i holding
// prompt.
func aiderInputEntry(i int, prompt string) string {
	return "\n# " + aiderStamp(i) + ".000000\n+" + prompt + "\n"
}

// aiderGrowth returns the aider growth set.
func aiderGrowth() growthSet {
	return growthSet{parser: "aider", cases: slices.Concat(aiderChatCases(), aiderSideCases())}
}

// aiderChatCases returns the cases of the chat history.
func aiderChatCases() []growthCase {
	return []growthCase{
		{kind: ".aider.chat.history.md", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			var chat, inputs strings.Builder
			chat.WriteString(aiderHeader(0))
			for i := range n {
				chat.WriteString("#### same prompt\n\nreply " + numbered("r", i) + "\n\n")
				inputs.WriteString(aiderInputEntry(1+i, "same prompt"))
			}
			return []growthFile{text(aiderChat, chat.String()), text(aiderInput, inputs.String())}
		}},
		{kind: ".aider.chat.history.md", shape: shapeManyFiles + " (many sessions, one input history)",
			files: func(t *testing.T, n int) []growthFile {
				var chat, inputs strings.Builder
				for i := range n {
					chat.WriteString(aiderHeader(10*i) + "#### " + numbered("prompt", i) + "\n\nreply\n\n")
					inputs.WriteString(aiderInputEntry(10*i+1, numbered("prompt", i)))
				}
				return []growthFile{text(aiderChat, chat.String()), text(aiderInput, inputs.String())}
			}},
		{kind: ".aider.chat.history.md", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			var chat strings.Builder
			chat.WriteString(aiderHeader(0) + "> Main model: " + long("model") + " with diff edit format\n\n")
			for i := range n {
				chat.WriteString("#### " + numbered("p", i) + "\n\n> Apply edit to a.py? (Y)es/(N)o [Yes]: y\n\n")
			}
			return one(aiderChat, []byte(chat.String()))
		}},
		{kind: ".aider.chat.history.md", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			var chat strings.Builder
			chat.WriteString(aiderHeader(0))
			for i := range n {
				chat.WriteString("# aider chat started at 2026-99-" + numbered("x", i) + "\n> \n####\n")
			}
			return one(aiderChat, []byte(chat.String()))
		}},
	}
}

// aiderSideCases returns the cases of the input and LLM histories.
func aiderSideCases() []growthCase {
	return []growthCase{
		{kind: ".aider.input.history", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			var chat, inputs strings.Builder
			for i := range n {
				chat.WriteString(aiderHeader(10*i) + "#### p\n")
				inputs.WriteString(aiderInputEntry(10*i+1, numbered("prompt", i)))
			}
			return []growthFile{text(aiderChat, chat.String()), text(aiderInput, inputs.String())}
		}},
		{kind: ".aider.input.history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			body := aiderInputEntry(0, "first") + string(lines(n, func(i int) string { return numbered("damaged ", i) }))
			return one(aiderInput, []byte(body))
		}},
		{kind: ".aider.llm.history", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			var chat, history strings.Builder
			for i := range n {
				chat.WriteString(aiderHeader(10*i) + "#### p\n")
				stamp := strings.Replace(aiderStamp(10*i+1), " ", "T", 1)
				history.WriteString("TO LLM " + stamp + "\n-------\nSYSTEM be brief\n-------\nUSER " +
					numbered("prompt", i) + "\nLLM RESPONSE " + stamp + "\nASSISTANT " + numbered("answer", i) + "\n")
			}
			return []growthFile{text(aiderChat, chat.String()), text(aiderLLM, history.String())}
		}},
		{kind: ".aider.llm.history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(aiderLLM, lines(n, func(i int) string { return numbered("before any marker ", i) }))
		}},
	}
}
