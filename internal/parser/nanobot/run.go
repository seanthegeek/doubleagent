package nanobot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// runRecord is what a cron or trigger run record gives: the prompt and its
// time, and the status row's text.
type runRecord struct {
	prompt string
	start  string
	status string
}

// parseRun emits the rows of a cron or trigger run record: the prompt as
// the session stored it (user) at the run's start, the reply (assistant),
// and a `cron run:` or `trigger run:` row with the run's status at
// updated_at_ms. A run record under a workspace, or in a project, has the
// directory above cron/ or triggers/ as its project.
func (p *Parser) parseRun(artifact *input.Artifact, emit parser.Emit) {
	match := p.files.run.FindStringSubmatch(artifact.Rel)
	kind := group(p.files.run, match, "kind")
	project := ""
	if group(p.files.run, match, "ws") != "" || (artifact.Home != nil && artifact.Home.Project) {
		if original := p.files.originalRun.FindStringSubmatch(artifact.Original); original != nil {
			project = original[1]
		}
	}
	record, problem, ok := p.loadObject(artifact, kind+" run record")
	if !ok {
		problem.ProjectPath = project
		emit(problem)
		return
	}
	end := timeutil.FromValue(record.Get("updated_at_ms"))
	run := triggerRun(record, end)
	if kind == "cron" {
		run = cronRun(record, end)
	}
	row := func(timestamp, turnType, text string) {
		out := p.settingRow(artifact, timestamp, text, parser.Str(record.Get("session_key")))
		out.TurnType = turnType
		out.ProjectPath = project
		emit(out)
	}
	if pytext.Strip(run.prompt) != "" {
		row(run.start, timeline.TurnUser, run.prompt)
	}
	if response := record.Get("response"); response.IsString() && response.Text() != "" {
		row(end, timeline.TurnAssistant, response.Text())
	}
	if failure := firstStr(record.Get("error"), record.Get("last_error")); failure != "" {
		run.status += " error=" + failure
	}
	row(end, timeline.TurnSystem, run.status)
}

// cronRun reads a cron run record. Its start is the epoch milliseconds in
// run_id, which is <job id>:<epoch ms at the run's start>:<hex>, else end.
func cronRun(record pyjson.Value, end string) runRecord {
	message := record.Get("prompt_vars").Get("message")
	prompt := parser.Str(record.Get("rendered_prompt"))
	if record.Get("job_name").Truthy() && message.IsString() {
		prompt = "Scheduled cron job triggered: " + record.Get("job_name").Str() + "\n\n" + message.Text()
	}
	runID := parser.Str(record.Get("run_id"))
	start := end
	if parts := rsplit(runID, ":", 2); len(parts) == 3 && isASCIIDigits(parts[1]) {
		start = ""
		if epoch, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			start = timeutil.FromInt(epoch)
		}
	}
	status := fmt.Sprintf("cron run: job %s (%s) run %s status=%s", parser.Str(record.Get("job_name")),
		parser.Str(record.Get("job_id")), runID, parser.Str(record.Get("status")))
	return runRecord{prompt: prompt, start: start, status: status}
}

// triggerRun reads a trigger run record, which starts at created_at_ms,
// else at end.
func triggerRun(record pyjson.Value, end string) runRecord {
	label := pytext.Strip(parser.Str(record.Get("trigger_name")))
	if label == "" {
		label = parser.Str(record.Get("trigger_id"))
	}
	prompt := "Local trigger received: " + label + "\n\n" + parser.Str(record.Get("content"))
	start := timeutil.FirstNonEmpty(timeutil.FromValue(record.Get("created_at_ms")), end)
	status := fmt.Sprintf("trigger run: trigger %s (%s) delivery %s status=%s attempts=%s",
		parser.Str(record.Get("trigger_name")), parser.Str(record.Get("trigger_id")),
		firstStr(record.Get("delivery_id"), record.Get("run_id")), parser.Str(record.Get("status")),
		getOr(record, "attempts", ""))
	return runRecord{prompt: prompt, start: start, status: status}
}

// rsplit splits text at the last n separators, as Python's str.rsplit.
func rsplit(text, separator string, n int) []string {
	var tail []string
	for range n {
		cut := strings.LastIndex(text, separator)
		if cut < 0 {
			break
		}
		tail = append([]string{text[cut+len(separator):]}, tail...)
		text = text[:cut]
	}
	return append([]string{text}, tail...)
}

// isASCIIDigits reports whether text is one or more ASCII digits.
func isASCIIDigits(text string) bool {
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}
