package analyze

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// Four times in order, as the epoch seconds an Agent Zero log item's
// timestamp holds: the first days of four months of one year, which the
// rows read back as the months 01, 03, 06 and 09.
const (
	january   = "1767225600"
	march     = "1772323200"
	june      = "1780272000"
	september = "1788220800"
)

// historyChat returns an Agent Zero chat.json whose main agent's history
// holds the messages and whose log holds the items, both JSON objects
// joined with commas.
func historyChat(messages, items string) string {
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` + messages + `]}}`
	return `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history).JSON() +
		`}],"log":{"logs":[` + items + `]}}`
}

// toolMessage returns a tool history message of the id naming the message
// file big.txt, with the tool result given when it is not "".
func toolMessage(messageID, result string) string {
	content := `{"tool_name":"x","file":"big.txt"`
	if result != "" {
		content += `,"tool_result":"` + result + `"`
	}
	return `{"_cls":"Message","id":"` + messageID + `","ai":false,"content":` + content + `}}`
}

// toolItem returns a tool log item of the id at the time, with its own
// content.
func toolItem(messageID, timestamp, content string) string {
	return `{"id":"` + messageID + `","type":"tool","timestamp":` + timestamp + `,"content":"` + content + `"}`
}

// fileChatRun runs timeline over a collection holding the chat and the
// message file big.txt, which holds FILE-TEXT, with the options given.
func fileChatRun(t *testing.T, chat string, options ...string) []string {
	t.Helper()
	return agentZeroRun(t, []collectedFile{agentZeroChat(chat),
		{"alice", "/home/alice", "agent-zero", "agent-zero/usr/chats/ctx/messages/big.txt", "FILE-TEXT"}},
		options...)
}

// tool returns the rows of one tool item in the month: its call, with no
// arguments, and its result.
func tool(month, result string) []string {
	return []string{month + " tool_use {}", month + " tool_result " + result}
}

// The text of a message file the history messages of several ids name is
// one text the tool items of all of them could show: it is shown once, in
// the first of them the filters keep, and the other items show their own
// content. A pattern finds every such item by the file's text as well as
// by its own content, and the items of one id carry it as their
// tool_use_id, so the filters keep every item of an id beside a kept one.
func TestAMessageFileSeveralIDsNameIsShownInTheFirstItemTheFilterKeeps(t *testing.T) {
	t.Parallel()
	twoIDs := historyChat(toolMessage("t1", "")+","+toolMessage("t2", ""),
		toolItem("t1", january, "first own")+","+toolItem("t2", june, "second own"))
	oneID := historyChat(toolMessage("t1", ""),
		toolItem("t1", january, "first own")+","+toolItem("t1", june, "second own"))
	fourItems := historyChat(toolMessage("t1", "")+","+toolMessage("t2", ""),
		toolItem("t1", january, "a1")+","+toolItem("t2", march, "b1")+","+toolItem("t1", june, "a2")+","+
			toolItem("t2", september, "b2"))
	tests := []struct {
		name, chat string
		options    []string
		want       [][]string
	}{
		{"two ids without a filter show the file in the first item", twoIDs, nil,
			[][]string{tool("01", "FILE-TEXT"), tool("06", "second own")}},
		{"two ids with a window keeping the later item show the file there", twoIDs,
			[]string{"--since", "2026-03-01"}, [][]string{tool("06", "FILE-TEXT")}},
		{"two ids with a window keeping the first item show the file there", twoIDs,
			[]string{"--until", "2026-03-01"}, [][]string{tool("01", "FILE-TEXT")}},
		{"two ids with a pattern the file matches show it in the first item alone", twoIDs,
			[]string{"--match", "FILE-TEXT"}, [][]string{tool("01", "FILE-TEXT")}},
		{"two ids with a pattern the later item's own content matches show the file there", twoIDs,
			[]string{"--match", "second own"}, [][]string{tool("06", "FILE-TEXT")}},
		{"one id without a filter shows the file in the first item", oneID, nil,
			[][]string{tool("01", "FILE-TEXT"), tool("06", "second own")}},
		{"one id with a window keeping the later item shows the file there", oneID,
			[]string{"--since", "2026-03-01"}, [][]string{tool("06", "FILE-TEXT")}},
		{"one id with a pattern the later item's own content matches keeps both items", oneID,
			[]string{"--match", "second own"}, [][]string{tool("01", "FILE-TEXT"), tool("06", "second own")}},
		{"two ids of two items each without a filter show the file in the first", fourItems, nil,
			[][]string{tool("01", "FILE-TEXT"), tool("03", "b1"), tool("06", "a2"), tool("09", "b2")}},
		{"a window from February shows the file in the other id's first item", fourItems,
			[]string{"--since", "2026-02-01"}, [][]string{tool("03", "FILE-TEXT"), tool("06", "a2"), tool("09", "b2")}},
		{"a window from April shows the file in the first id's later item", fourItems,
			[]string{"--since", "2026-04-01"}, [][]string{tool("06", "FILE-TEXT"), tool("09", "b2")}},
		{"a window from July shows the file in the last item", fourItems,
			[]string{"--since", "2026-07-01"}, [][]string{tool("09", "FILE-TEXT")}},
		{"a window to February keeps the file in the first item", fourItems,
			[]string{"--until", "2026-02-01"}, [][]string{tool("01", "FILE-TEXT")}},
		{"a pattern only the last item's own content matches shows the file in its id's first", fourItems,
			[]string{"--match", "b2"}, [][]string{tool("03", "FILE-TEXT"), tool("09", "b2")}},
		{"a pattern the file matches shows it in the first item", fourItems,
			[]string{"--match", "FILE-TEXT"}, [][]string{tool("01", "FILE-TEXT"), tool("06", "a2")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := fileChatRun(t, test.chat, test.options...)
			if want := slices.Concat(test.want...); !slices.Equal(rows, want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
			}
			if count := strings.Count(strings.Join(rows, "\n"), "FILE-TEXT"); count != 1 {
				t.Errorf("the file's text is shown %d times, want once", count)
			}
		})
	}
}

// A tool result in the history message of an id whose message file
// another id's item shows is shown in the first item of that id, as the
// result of an id shown with no file; when the file is shown in an item
// of the id itself, the file stands for the result.
func TestAToolResultBesideAFileAnotherIDShowsIsShownInTheFirstItemOfItsID(t *testing.T) {
	chat := historyChat(toolMessage("t1", "")+","+toolMessage("t2", "SECOND-RESULT"),
		toolItem("t1", january, "a1")+","+toolItem("t2", march, "b1")+","+toolItem("t2", june, "b2"))
	tests := []struct {
		name    string
		options []string
		want    [][]string
	}{
		{"without a filter the result is in the id's first item", nil,
			[][]string{tool("01", "FILE-TEXT"), tool("03", "SECOND-RESULT"), tool("06", "b2")}},
		{"a window keeping the id's items shows the file in its first", []string{"--since", "2026-02-01"},
			[][]string{tool("03", "FILE-TEXT"), tool("06", "b2")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := fileChatRun(t, chat, test.options...)
			if want := slices.Concat(test.want...); !slices.Equal(rows, want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// userMessage returns a user history message of the id with the prompt.
func userMessage(messageID, prompt string) string {
	return `{"_cls":"Message","id":"` + messageID + `","ai":false,"content":{"user_message":"` + prompt + `"}}`
}

// userItem returns a user log item of the id at the time, with its own
// content.
func userItem(messageID, timestamp, content string) string {
	return `{"id":"` + messageID + `","type":"user","timestamp":` + timestamp + `,"content":"` + content + `"}`
}

// A history message whose id an earlier message has gives its own rows,
// at the time of the log item its id's rows go with: the first item of
// the id the filters keep. The earlier message joins the item as before.
func TestALaterHistoryMessageOfAnIDIsShownAtALogItemOfTheID(t *testing.T) {
	t.Parallel()
	messages := userMessage("u", "FIRST-PROMPT") + "," + userMessage("u", "LATER-PROMPT")
	oneItem := historyChat(messages, userItem("u", june, ""))
	twoItems := historyChat(messages, userItem("u", january, "")+","+userItem("u", june, "june ask"))
	tests := []struct {
		name, chat string
		options    []string
		want       []string
	}{
		{"one item without a filter", oneItem, nil, []string{"06 user FIRST-PROMPT", "06 user LATER-PROMPT"}},
		{"one item in a window", oneItem, []string{"--since", "2026-03-01"},
			[]string{"06 user FIRST-PROMPT", "06 user LATER-PROMPT"}},
		{"one item with a pattern only the later message matches", oneItem, []string{"--match", "LATER"},
			[]string{"06 user LATER-PROMPT"}},
		{"two items without a filter take the first item's time", twoItems, nil,
			[]string{"01 user FIRST-PROMPT", "01 user LATER-PROMPT", "06 user june ask"}},
		{"two items with a window keeping the later item take its time", twoItems,
			[]string{"--since", "2026-03-01"}, []string{"06 user FIRST-PROMPT", "06 user LATER-PROMPT"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := agentZeroRun(t, []collectedFile{agentZeroChat(test.chat)}, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
