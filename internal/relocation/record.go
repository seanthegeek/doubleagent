package relocation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// recordFile records a hint of a file, whose path and value are canonical
// paths the log shows in native form.
func (run *search) recordFile(user, file string, line int, variable, value, reason string) error {
	return run.recordShown(user, file, line, variable, value, reason, syspath.ToNative)
}

// record records a hint of a registry value, whose key path and value the
// log shows as they are.
func (run *search) record(user, file string, line int, variable, value, reason string) error {
	return run.recordShown(user, file, line, variable, value, reason, verbatim)
}

// verbatim returns text unchanged.
func verbatim(text string) string { return text }

// recordShown adds one hint per RELOCATION_VARS line naming variable, in table
// order, unless the same user, file, line, variable and agent is already
// held, and logs it after the base's header, which is logged once: the line
// ends ` = VALUE` for a resolved value, `; value not resolved: REASON` for a
// refused one, with file and value as shown gives them. A value also joins
// Values and the resolved count.
func (run *search) recordShown(user, file string, line int, variable, value, reason string,
	shown func(string) string) error {
	for _, entry := range run.deps.Tables.RelocationVars {
		if entry.Variable != variable {
			continue
		}
		key := strings.Join([]string{user, file, strconv.Itoa(line), variable, entry.Agent}, "|")
		if run.seen[key] {
			continue
		}
		run.seen[key] = true
		hint := Hint{User: user, Source: file, Line: line, Variable: variable, Agent: entry.Agent}
		if value != "" {
			hint.Value = value
			run.result.Values = append(run.result.Values, Value{User: user, Variable: variable,
				Agent: entry.Agent, Replaces: entry.Replaces, Value: value})
			run.result.Resolved++
		}
		run.result.Hints = append(run.result.Hints, hint)
		if err := run.log(hintLine(hint, reason, shown)); err != nil {
			return err
		}
	}
	return nil
}

// log writes the pending header, once, and then text.
func (run *search) log(text string) error {
	if run.header != "" {
		header := run.header
		run.header = ""
		if err := run.deps.Log.Line(header); err != nil {
			return fmt.Errorf("log relocation header: %w", err)
		}
	}
	if err := run.deps.Log.Line(text); err != nil {
		return fmt.Errorf("log relocation hint: %w", err)
	}
	return nil
}

// hintLine is the log line of a hint: `relocation hint: FILE:LINE names
// VARIABLE (agent)`, FILE alone for line 0, then the value or the reason;
// FILE and the value are written as shown gives them.
func hintLine(hint Hint, reason string, shown func(string) string) string {
	where := shown(hint.Source)
	if hint.Line != 0 {
		where += ":" + strconv.Itoa(hint.Line)
	}
	text := "relocation hint: " + where + " names " + hint.Variable + " (" + hint.Agent + ")"
	switch {
	case hint.Value != "":
		return text + " = " + shown(hint.Value)
	case reason != "":
		return text + "; value not resolved: " + reason
	}
	return text
}

// isDaemonPath reports a path under a LaunchDaemons directory, whose plist
// names the account launchd runs it as.
func isDaemonPath(path string) bool {
	return glob.MatchPath("*/LaunchDaemons/*", path)
}

// homeHeader is the log line heading the hints of a home.
func homeHeader(home users.Home) string {
	return "Relocation hints for " + home.User + " (" + syspath.ToNative(home.Path) + ")"
}

// projectHeader is the log line heading the hints of a project.
func projectHeader(project users.Home) string {
	return "Relocation hints in project " + syspath.ToNative(project.Path)
}
