package zed

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// repeatedCallThread returns a thread whose one Agent message holds calls
// ToolUse parts that all name the id "a", whose result's content is size
// bytes.
func repeatedCallThread(calls, size int) string {
	parts := strings.Repeat(`{"ToolUse":{"id":"a","name":"x"}},`, calls)
	return `{"messages":[{"Agent":{"content":[` + strings.TrimSuffix(parts, ",") +
		`],"tool_results":{"a":{"content":"` + strings.Repeat("R", size) + `"}}}}]}`
}

func TestAToolResultFollowsOnlyTheFirstCallOfItsID(t *testing.T) {
	const size = 10000
	for _, calls := range []int{1000, 4000} {
		t.Run(fmt.Sprintf("%d calls", calls), func(t *testing.T) {
			blob := repeatedCallThread(calls, size)
			var problems parser.Problems
			items := New().messages(decoded(t, blob), 7, &problems)
			uses, results, text := 0, 0, 0
			for _, each := range items {
				text += len(each.text)
				switch each.turnType {
				case timeline.TurnToolUse:
					uses++
				case timeline.TurnToolResult:
					results++
				}
			}
			if second := items[1]; second.turnType != timeline.TurnToolResult || second.toolUseID != "a" ||
				len(second.text) != size {
				t.Errorf("the second item is %s %s of %d bytes; want the result of a, %d bytes",
					second.turnType, second.toolUseID, len(second.text), size)
			}
			t.Logf("%d calls: input %d bytes, item text %d bytes", calls, len(blob), text)
			if uses != calls || results != 1 || len(items) != calls+1 || len(problems) != 0 {
				t.Errorf("got %d calls, %d results, %d items, problems %v; want %d calls and one result",
					uses, results, len(items), problems, calls)
			}
			if text > len(blob) {
				t.Errorf("the items hold %d bytes of text, more than the %d bytes of the thread", text, len(blob))
			}
		})
	}
}
