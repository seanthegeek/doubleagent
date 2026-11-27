package kiro

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// exportRows writes text as a /save export in a new home and returns the
// rows the parser gives for it.
func exportRows(t *testing.T, text string) []timeline.Row {
	t.Helper()
	const rel = "proj/chat.json"
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "kiro",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// The tool a deny-rule prompt names goes into the decision row of each of
// the message's results, so a name over the limit is cut once for the
// message and written cut in every one of them, with one problem for the
// conversation: the prompt's own row holds the name whole once, and twice
// the results cost twice the bytes, never the name's length again.
func TestADenyRuleToolIsCutInTheDecisionOfEveryResult(t *testing.T) {
	long := "t" + strings.Repeat("a", 50_000)
	prompt := "Tool use with " + long + " was rejected because the arguments supplied were forbidden"
	var sizes []int
	for _, n := range []int{20, 40} {
		t.Run(fmt.Sprintf("%d results", n), func(t *testing.T) {
			var results []string
			for i := range n {
				results = append(results, `{"tool_use_id": "u`+strconv.Itoa(i)+`", "content": []}`)
			}
			rows := exportRows(t, `{"conversation_id": "c", "history": [{"user": {"content": {"CancelledToolUses": `+
				`{"prompt": "`+prompt+`", "tool_use_results": [`+strings.Join(results, ",")+`]}}}}]}`)
			decisions, problems, whole, size := 0, 0, 0, 0
			for _, row := range rows {
				for _, field := range row.Strings() {
					size += len(field)
				}
				if strings.HasPrefix(row.Text, "permission decision: denied by=deny-rule tool=t") {
					decisions++
				}
				if row.Text == parserkit.ProblemPrefix+parserkit.CarriedProblem("deny rule tool", 1) {
					problems++
				}
				if strings.Contains(row.Text, long) {
					whole++
				}
			}
			sizes = append(sizes, size)
			if decisions != n || problems != 1 || whole != 1 {
				t.Errorf("%d decisions, %d problems and %d whole names, want %d, 1 and 1", decisions, problems,
					whole, n)
			}
		})
	}
	if growth := sizes[1] - sizes[0]; growth > 20*(2*parserkit.CarriedLimit+1024) {
		t.Errorf("20 more results cost %d bytes", growth)
	}
}
