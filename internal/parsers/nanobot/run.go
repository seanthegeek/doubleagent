package nanobot

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
func (p *Parser) parseRun(artifact *input.Artifact, emit parserkit.Emit) {
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
		out := p.settingRow(artifact, timestamp, text, parserkit.Text(record.Get("session_key")))
		out.TurnType = turnType
		out.ProjectPath = project
		emit(out)
	}
	if strings.TrimSpace(run.prompt) != "" {
		row(run.start, timeline.TurnUser, run.prompt)
	}
	if response := record.Get("response"); response.IsString() && response.StringValue() != "" {
		row(end, timeline.TurnAssistant, response.StringValue())
	}
	if failure := parserkit.Text(parserkit.FirstNonEmpty(record.Get("error"), record.Get("last_error"))); failure != "" {
		run.status += " error=" + failure
	}
	row(end, timeline.TurnSystem, run.status)
}

// cronRun reads a cron run record. Its start is the epoch milliseconds in
// run_id, which is <job id>:<epoch ms at the run's start>:<hex>, else end.
func cronRun(record jsonvalue.Value, end string) runRecord {
	message := record.Get("prompt_vars").Get("message")
	prompt := parserkit.Text(record.Get("rendered_prompt"))
	if !record.Get("job_name").IsEmpty() && message.IsString() {
		prompt = "Scheduled cron job triggered: " + record.Get("job_name").Text() + "\n\n" + message.StringValue()
	}
	runID := parserkit.Text(record.Get("run_id"))
	start := end
	if parts := strings.Split(runID, ":"); len(parts) >= 3 && isASCIIDigits(parts[len(parts)-2]) {
		start = ""
		if epoch, err := strconv.ParseInt(parts[len(parts)-2], 10, 64); err == nil {
			start = timeutil.FromInt(epoch)
		}
	}
	status := "cron run: job " + parserkit.Text(record.Get("job_name")) + " (" +
		parserkit.Text(record.Get("job_id")) + ") run " + runID + " status=" +
		parserkit.Text(record.Get("status"))
	return runRecord{prompt: prompt, start: start, status: status}
}

// triggerRun reads a trigger run record, which starts at created_at_ms,
// else at end.
func triggerRun(record jsonvalue.Value, end string) runRecord {
	label := strings.TrimSpace(parserkit.Text(record.Get("trigger_name")))
	if label == "" {
		label = parserkit.Text(record.Get("trigger_id"))
	}
	prompt := "Local trigger received: " + label + "\n\n" + parserkit.Text(record.Get("content"))
	start := timeutil.FirstNonEmpty(timeutil.FromValue(record.Get("created_at_ms")), end)
	status := "trigger run: trigger " + parserkit.Text(record.Get("trigger_name")) + " (" +
		parserkit.Text(record.Get("trigger_id")) + ") delivery " +
		parserkit.Text(parserkit.FirstNonEmpty(record.Get("delivery_id"), record.Get("run_id"))) +
		" status=" + parserkit.Text(record.Get("status")) + " attempts=" +
		getOr(record, "attempts", "")
	return runRecord{prompt: prompt, start: start, status: status}
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
