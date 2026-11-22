package openclaw

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// execRowTexts runs execDocument on the approvals file text, a stored row
// with rowid 4, and returns the texts of the rows it emits.
func execRowTexts(t *testing.T, text string) []string {
	t.Helper()
	document, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	artifact := &input.Artifact{Rel: ".openclaw/state/openclaw.sqlite", Agent: "openclaw",
		Manifest: jsonvalue.NewNull()}
	rows := &stateRows{Parser: New(), artifact: artifact,
		emit: func(row timeline.Row) { texts = append(texts, row.Text) }}
	rows.execDocument(4, document, "")
	return texts
}

// approvalsFile returns an approvals file with a socket token of size
// bytes and size allowlist entries of the default policy, and the exact
// number of bytes of row text it gives: a fixed cost per entry and the
// token's row once.
func approvalsFile(size int) (string, int) {
	const source = " (from state/openclaw.sqlite exec_approvals_config, updated_at_ms)"
	token := "FAKE-TOKEN-" + strings.Repeat("t", size)
	entries := make([]string, size)
	want := len("permission setting: socket.token=") + len(token) + len(source)
	for i := range entries {
		pattern := "p" + strconv.Itoa(i)
		entries[i] = `{"pattern": "` + pattern + `"}`
		want += len("permission setting: defaults.allowlist=") + len(pattern) + len(" source=-") + len(source)
	}
	text := `{"socket": {"token": "` + token + `"}, "defaults": {"allowlist": [` +
		strings.Join(entries, ", ") + `]}}`
	return text, want
}

func TestTheApprovalsSocketTokenIsOneRowHoweverManySettingsTheFileHolds(t *testing.T) {
	for _, size := range []int{100, 400} {
		text, want := approvalsFile(size)
		texts := execRowTexts(t, text)
		got, tokens := 0, 0
		for _, row := range texts {
			got += len(row)
			tokens += strings.Count(row, "FAKE-TOKEN-")
		}
		if got != want || len(texts) != size+1 || tokens != 1 {
			t.Errorf("size %d: %d rows of %d bytes holding the token %d times, want %d rows of %d bytes "+
				"holding it once", size, len(texts), got, tokens, size+1, want)
		}
		if last := texts[len(texts)-1]; !strings.HasPrefix(last, "permission setting: socket.token=FAKE-TOKEN-") {
			t.Errorf("size %d: the last row is %.80q, want the token's row", size, last)
		}
	}
}
